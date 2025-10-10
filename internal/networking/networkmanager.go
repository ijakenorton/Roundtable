package networking

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

var (
	listeningConnectionExists error = errors.New("manager already has an active listening connection")
)

// WebRTCConnectionManager handles networking in the application using WebRTC
//
// Specifically, once instantiated, the WebRTCConnectionManager handles listening for connections,
// accepting new connections, passing connections back to be stored with the peer.
//
// Note that this class *only* handles creating connections, both offering and answering (to use the WebRTC terminology).
// Actually sending/receiving data on those connections should be handled by the webrtc.PeerConnections themselves,
// and closing those connections is handled by the Peer object under github.com/hmcalister/roundtable/internal/peer/Peer
//
// The general flow of connections is as follows:
//
//  1. The application prints a BASE64 encoded "identifier string": the publicly available IP address + port number
//     (and any tokens for application level security) to the local user.
//
//  2. The local user sends that string to any number of remote peers *over a trusted channel*.
//
//  3. The remote peers paste the string into their running applications, which prompts a call to WebRTCConnectionManager.Dial
//     The offered connection session description protocol (SDP) is sent to the local application
//     *via a public signalling server* (see github.com/hmcalister/roundtable/cmd/signallingserver).
//
//  4. The local client gets a new connection offer on the incomingOfferHTTPServer, creates a listening WebRTCConnectionManager.PeerConnection,
//     responds to the HTTP request with a new SDP, and waits for the connection to be finalized on the new PeerConnection.
//
// 4a. Any tokens sent alongside the first encoded string are used to validate application-level security. This is a TODO.
//
//  5. The newly established and connected PeerConnection is returned along WebRTCConnectionManager.IncomingConnectionChannel,
//     and a new listeningConnection is made, ready to listen for another peer.
//
// In this way, a user need only publicly broadcast one piece of information which all remote peers can use to make a connection.
// Security is still ensured, with separate keys per p2p connection, but without a horrendous user experience.
//
// As a side note, the newly formed connection should first send the exchange the identifier string, both to user as a
// unique ID for the peer, and so that the string can be forwarded to new connections such that a user need only "connect"
// to one user in the chat room, with the application connecting to all other users automatically
// (once informed of their identifier strings)
type WebRTCConnectionManager struct {
	logger *slog.Logger

	connectionConfiguration webrtc.Configuration
	connectionOfferOptions  webrtc.OfferOptions
	connectionAnswerOptions webrtc.AnswerOptions

	// TODO Extract this to a ConnectRPC framework
	incomingSDPOfferServer *http.ServeMux

	// A channel to return established incoming connections
	//
	// Once instantiated with NewWebRTCConnectionManager, the caller should listen on
	// this channel for new connections, as this signals a peer has dialed, authenticated,
	// and is ready to send data. A value on this channel indicated a new Peer should be made.
	IncomingConnectionChannel chan<- *webrtc.PeerConnection
}

// Create a new WebRTCConnectionManager.
//
// connectionConfiguration defines the configuration to use for all webrtc.PeerConnections made by this client, both offering and answering.
// connectionOfferOptions defines the configurations to use for only the offering connections.
// connectionAnswerOptions defines the configurations to use for only the answering connections.
// See https://github.com/pion/webrtc for details on these options.
//
// logger allows for a child logger to be used specifically for this client. Create a child logger like:
// ```go
// childLogger := slog.Default().With(
//
//	slog.Group("WebRTCConnectionManager"),
//
// )
// ```
// If no logger is given, slog.Default() is used.
func NewWebRTCConnectionManager(
	connectionConfig webrtc.Configuration,
	connectionOfferOptions webrtc.OfferOptions,
	connectionAnswerOptions webrtc.AnswerOptions,
	logger *slog.Logger,
) *WebRTCConnectionManager {
	if logger == nil {
		logger = slog.Default()
	}

	incomingSDPOfferServer := http.NewServeMux()
	manager := &WebRTCConnectionManager{
		logger:                    logger,
		connectionConfiguration:   connectionConfig,
		connectionOfferOptions:    connectionOfferOptions,
		connectionAnswerOptions:   connectionAnswerOptions,
		incomingSDPOfferServer:    incomingSDPOfferServer,
		IncomingConnectionChannel: make(chan<- *webrtc.PeerConnection),
	}

	incomingSDPOfferServer.HandleFunc("/signal", manager.listenIncomingSessionOffers)

	return manager
}

// Listen for an incoming SDP offer on HTTP.
//
// Uses the public signalling server to forward traffic back and forth to remote peer.
//
// When a new offer is received, this method starts a new listening webrtc.PeerConnection, generates an
// answer to the offer, replies (via the signalling server) with the answer, and waits for the connection.
//
// Once the connection is established, the webrtc.PeerConnection is sent along the IncomingConnectionChannel.
//
// If the connection cannot be initialized, cannot be established, or if the context is canceled, this method returns an error code.
func (manager *WebRTCConnectionManager) listenIncomingSessionOffers(w http.ResponseWriter, r *http.Request) {
	var signallingOffer SignallingOffer
	if err := json.NewDecoder(r.Body).Decode(&signallingOffer); err != nil {
			"error while decoding new session offer from JSON",
			"err", err,
			"request", r,
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pc, err := webrtc.NewPeerConnection(manager.connectionConfiguration)
	if err != nil {
		slog.Error(
			"error while creating new peer connection for listening",
			"err", err,
			"connection config", manager.connectionConfiguration,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {})
	})

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		slog.Error(
			"error while creating answer of new peer connection",
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		slog.Error(
			"error while setting local description of new peer connection",
			"err", err,
			"answer", answer,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		slog.Error(
			"error while setting remote description of new peer connection",
			"err", err,
			"offer", offer,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	// Wait for ICE to resolve, finalizing connection
	<-webrtc.GatheringCompletePromise(pc)

	answerJSON, err := json.Marshal(pc.LocalDescription())
	if err != nil {
		slog.Error(
			"error while marshalling local description of new peer connection to JSON",
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(answerJSON)
}

// Attempt to make a connection to a peer. Returns a non-nil error if connection is not successful.
// If connection is successful, then the connection is added to the NetworkManager list of managed connections.
//
// This function creates a new webrtc.PeerConnection, so listening is not disrupted during dialing.
// The returned connection is owned by the caller, meaning it should be closed by the called, too.
func (manager *WebRTCConnectionManager) Dial(remoteAddress string) (*webrtc.PeerConnection, error) {
	decoded, err := base64.StdEncoding.DecodeString(remoteAddress)
	if err != nil {
		slog.Error(
			"error while decoding remote address",
			"err", err,
		)
		return nil, err
	}
	remoteEndpoint := string(decoded)

	pc, err := webrtc.NewPeerConnection(manager.connectionConfiguration)
	if err != nil {
		slog.Error(
			"error while creating new peer connection for dialing",
			"err", err,
			"connection config", manager.connectionConfiguration,
		)
		return nil, err
	}

	offer, err := pc.CreateOffer(&manager.connectionOfferOptions)
	if err != nil {
		slog.Error(
			"error while creating new offer in dialing",
			"err", err,
			"connection offer config", manager.connectionOfferOptions,
		)
		pc.Close()
		return nil, err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		slog.Error(
			"error while setting connection local description in dialing",
			"err", err,
			"offer", offer,
		)
		pc.Close()
		return nil, err
	}

	offerJSON, err := json.Marshal(offer)
	if err != nil {
		slog.Error(
			"error while marshalling offer to JSON",
			"err", err,
		)
		pc.Close()
		return nil, err
	}

	resp, err := http.Post(remoteEndpoint, "application/json", bytes.NewBuffer(offerJSON))
	if err != nil {
		slog.Error(
			"error while posting offer to remote server",
			"err", err,
			"offerJSON", offerJSON,
		)
		pc.Close()
		return nil, err
	}
	defer resp.Body.Close()

	var answer webrtc.SessionDescription
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		slog.Error(
			"error while parsing answer response from remote peer",
			"err", err,
		)
		pc.Close()
		return nil, err
	}

	if err = pc.SetRemoteDescription(answer); err != nil {
		slog.Error(
			"error while setting connection local description in dialing",
			"err", err,
			"answer", answer,
		)
		pc.Close()
		return nil, err
	}

	return pc, nil
}

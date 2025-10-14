package networking

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/peer"
	"github.com/pion/webrtc/v4"
)

const (
	// Defines the endpoint that clients and the signalling server listens on for request.
	//
	// The endpoint is defined by fmt.Sprintf("%s/%s", localaddress, networking.SIGNAL_ENDPOINT)
	SIGNAL_ENDPOINT = "signal"
)

// WebRTCConnectionManager handles networking in the application using WebRTC
//
// Specifically, once instantiated, the WebRTCConnectionManager handles listening for connections,
// accepting new connections, passing connections back to be stored with the peer.
//
// Note that this class *only* handles creating connections, both offering and answering (to use the WebRTC terminology).
//
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
//     via a public signalling server (see github.com/hmcalister/roundtable/cmd/signallingserver).
//
//  4. The local client gets a new connection offer on the incomingOfferHTTPServer, creates a listening WebRTCConnectionManager.PeerConnection,
//     responds to the HTTP request with a new SDP, and waits for the connection to be finalized on the new PeerConnection.
//
// 4a. Any tokens sent alongside the first encoded string are used to validate application-level security. This is a TODO.
//
//  5. The dialling client gets the response, finalizes the PeerConnection, and returns the newly established connection
//     to the caller of Dial. The listening client takes the established and connected PeerConnection and feeds it
//     into WebRTCConnectionManager.IncomingConnectionChannel, ready to be handled by the main program.
//
// In this way, a user need only publicly broadcast one piece of information which all remote peers can use to make a connection.
// Security is still ensured, with separate keys per p2p connection, but without a horrendous user experience of per-user transmissions of information.
//
// As a side note, the newly formed connection should first send the exchange the identifier string, both to user as a
// unique ID for the peer, and so that the string can be forwarded to new connections such that a user need only "connect"
// to one user in the chat room, with the application connecting to all other users automatically
// (once informed of their identifier strings)
// This is, again, a TODO.
type WebRTCConnectionManager struct {
	logger *slog.Logger

	// The URL (address and endpoint) for the signalling server to set up connections
	signallingServerURL string

	// Factory to create new Peers during Dial and answering.
	// The PeerFactory handles setting up the webrtc.PeerConnection with
	// datachannels and audio tracks.
	peerFactory *peer.PeerFactory

	webrtcAPI               *webrtc.API
	connectionConfiguration webrtc.Configuration
	connectionOfferOptions  webrtc.OfferOptions
	connectionAnswerOptions webrtc.AnswerOptions

	// TODO Extract this to a ConnectRPC framework
	incomingSDPOfferServer *http.ServeMux

	// A channel to return established incoming connections
	//
	// Once instantiated with NewWebRTCConnectionManager, the caller should listen on
	// this channel for new connections, as this signals a peer has dialed, authenticated,
	// and is ready to send data.
	IncomingConnectionChannel chan *peer.Peer
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
	localport int,
	signallingServerAddress string,
	peerFactory *peer.PeerFactory,
	connectionConfig webrtc.Configuration,
	connectionOfferOptions webrtc.OfferOptions,
	connectionAnswerOptions webrtc.AnswerOptions,
	logger *slog.Logger,
) *WebRTCConnectionManager {
	if logger == nil {
		logger = slog.Default()
	}

	mediaEngine := &webrtc.MediaEngine{}
	// TODO: Register all codecs, include user defined ones?
	// mediaEngine.RegisterDefaultCodecs()
	for i, codec := range []webrtc.RTPCodecCapability{
		CodecOpus48000Mono,
		CodecOpus48000Stereo,
	} {
		err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: codec,
			PayloadType:        webrtc.PayloadType(100 + i), // See https://www.iana.org/assignments/rtp-parameters/rtp-parameters.xhtml
		}, webrtc.RTPCodecTypeAudio)
		if err != nil {
			logger.Error("error while registering codec", "codec", codec, "err", err)
		}
	}
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
	)

	incomingSDPOfferServer := http.NewServeMux()
	manager := &WebRTCConnectionManager{
		logger:                    logger,
		signallingServerURL:       fmt.Sprintf("%s/%s", signallingServerAddress, SIGNAL_ENDPOINT),
		peerFactory:               peerFactory,
		webrtcAPI:                 api,
		connectionConfiguration:   connectionConfig,
		connectionOfferOptions:    connectionOfferOptions,
		connectionAnswerOptions:   connectionAnswerOptions,
		incomingSDPOfferServer:    incomingSDPOfferServer,
		IncomingConnectionChannel: make(chan *peer.Peer),
	}

	incomingSDPOfferServer.HandleFunc(
		fmt.Sprintf("POST /%s", SIGNAL_ENDPOINT),
		manager.listenForSessionOffers,
	)
	go http.ListenAndServe(fmt.Sprintf("localhost:%d", localport), incomingSDPOfferServer)

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
func (manager *WebRTCConnectionManager) listenForSessionOffers(w http.ResponseWriter, r *http.Request) {
	requestLogger := manager.logger.WithGroup("request").With(
		"requestUUID", uuid.New().String(),
	)
	requestLogger.Debug("new incoming session offer")

	// --------------------------------------------------------------------------------
	// Decode the offer

	// TODO: Likely a security risk to read the body... what if the body is very large?
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		requestLogger.Error(
			"error while reading request body",
			"err", err,
			"request", r,
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var signallingOffer SignallingOffer
	if err := json.Unmarshal(requestBody, &signallingOffer); err != nil {
		requestLogger.Error(
			"error while decoding new session offer from JSON",
			"err", err,
			"request", r,
			"requestBody", requestBody,
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	requestLogger = requestLogger.With("offerUUID", signallingOffer.OfferUUID.String())
	requestLogger.Info("session offer received")

	// --------------------------------------------------------------------------------
	// Establish a new connection to set up this half of the PeerConnection

	pc, err := manager.webrtcAPI.NewPeerConnection(manager.connectionConfiguration)
	if err != nil {
		requestLogger.Error(
			"error while creating new peer connection for listening",
			"err", err,
			"connection config", manager.connectionConfiguration,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	requestLogger.Debug("peer connection started")

	answeringPeer, err := manager.peerFactory.NewAnsweringPeer(pc)
	if err != nil {
		requestLogger.Error(
			"error while creating new answering peer from factory",
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// --------------------------------------------------------------------------------
	// Create the answer to the incoming offer, set the values on this half of the PeerConnection

	if err := pc.SetRemoteDescription(signallingOffer.WebRTCSessionDescription); err != nil {
		requestLogger.Error(
			"error while setting remote description of new peer connection",
			"err", err,
			"signallingOffer", signallingOffer,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		requestLogger.Error(
			"error while creating answer of new peer connection",
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		requestLogger.Error(
			"error while setting local description of new peer connection",
			"err", err,
			"answer", answer,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	requestLogger.Debug("answering peer connection initialized")

	// Wait for ICE to resolve, finalizing connection
	<-webrtc.GatheringCompletePromise(pc)
	requestLogger.Debug("answering peer connection ICE resolved")

	// --------------------------------------------------------------------------------
	// Respond to the signalling server with our answer and wait...

	signallingAnswer := SignallingAnswer{
		OfferUUID:                signallingOffer.OfferUUID,
		WebRTCSessionDescription: *pc.LocalDescription(),
	}
	signallingAnswerJSON, err := json.Marshal(signallingAnswer)
	if err != nil {
		requestLogger.Error(
			"error while marshalling local description of new peer connection to JSON",
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		pc.Close()
		return
	}

	requestLogger.Debug("sending answer", "signallingAnswerJSON", signallingAnswerJSON)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(signallingAnswerJSON)

	// --------------------------------------------------------------------------------
	// TODO: handle final connections, then forward on incomingConnectionChannel

	requestLogger.Info("peer connection established")
	manager.IncomingConnectionChannel <- answeringPeer
}

// Attempt to make a connection to a peer. Returns a non-nil error if connection is not successful.
// If connection is successful, then the connection is returned to be owned by the caller.
//
// The returned connection is owned by the caller, meaning it should be closed by the called, too.
func (manager *WebRTCConnectionManager) Dial(ctx context.Context, remoteEndpointEncoded string) (*peer.Peer, error) {
	offerUUID := uuid.New()
	requestLogger := manager.logger.WithGroup("request").With(
		"requestUUID", uuid.New().String(),
		"offerUUID", offerUUID.String(),
		"remoteAddress", remoteEndpointEncoded,
	)
	requestLogger.Info("new SDP offer started")

	// --------------------------------------------------------------------------------
	// Decode the given remote endpoint string to use for the remote peer

	decoded, err := base64.StdEncoding.DecodeString(remoteEndpointEncoded)
	if err != nil {
		requestLogger.Error(
			"error while decoding remote address",
			"err", err,
		)
		return nil, err
	}
	remoteEndpoint := string(decoded)
	requestLogger = requestLogger.With("remoteEndpoint", remoteEndpoint)

	// --------------------------------------------------------------------------------
	// Establish this side of the PeerConnection

	pc, err := manager.webrtcAPI.NewPeerConnection(manager.connectionConfiguration)
	if err != nil {
		requestLogger.Error(
			"error while creating new peer connection for dialing",
			"err", err,
			"connection config", manager.connectionConfiguration,
		)
		return nil, err
	}

	offeringPeer, err := manager.peerFactory.NewOfferingPeer(pc)
	if err != nil {
		requestLogger.Error(
			"error while creating new offering peer from factory",
			"err", err,
		)
		return nil, err
	}

	// --------------------------------------------------------------------------------
	// Create a new offer, set our side of the PeerConnection

	offer, err := pc.CreateOffer(&manager.connectionOfferOptions)
	if err != nil {
		requestLogger.Error(
			"error while creating new offer in dialing",
			"err", err,
			"connection offer config", manager.connectionOfferOptions,
		)
		pc.Close()
		return nil, err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		requestLogger.Error(
			"error while setting connection local description in dialing",
			"err", err,
			"offer", offer,
		)
		pc.Close()
		return nil, err
	}

	// --------------------------------------------------------------------------------
	// Embed our offer in a SignallingOffer struct, send this to the signalling server, and wait for a response

	signallingOffer := SignallingOffer{
		RemoteEndpoint:           remoteEndpoint,
		OfferUUID:                offerUUID,
		WebRTCSessionDescription: offer,
	}
	signallingOfferJSON, err := json.Marshal(signallingOffer)
	if err != nil {
		requestLogger.Error(
			"error while marshalling offer to JSON",
			"err", err,
		)
		pc.Close()
		return nil, err
	}
	requestLogger.Debug("sending offer to signalling server", "signallingOfferJSON", signallingOfferJSON)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		manager.signallingServerURL,
		bytes.NewBuffer(signallingOfferJSON),
	)
	if err != nil {
		requestLogger.Error(
			"error while creating new http request",
			"err", err,
			"signallingOfferJSON", signallingOfferJSON,
		)
		pc.Close()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// If ctx.cancel is called, or ctx timeout is reached, this returns with non-nil error
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		requestLogger.Error(
			"error while posting offer to remote server",
			"err", err,
			"signallingOfferJSON", signallingOfferJSON,
		)
		pc.Close()
		return nil, err
	}
	defer resp.Body.Close()
	requestLogger.Debug("response received from signalling server")

	// --------------------------------------------------------------------------------
	// Read the incoming signalling answer, decode it, and set the remote side of our PeerConnection

	var signallingAnswer SignallingAnswer
	if err := json.NewDecoder(resp.Body).Decode(&signallingAnswer); err != nil {
		requestLogger.Error(
			"error while parsing answer response from remote peer",
			"err", err,
		)
		pc.Close()
		return nil, err
	}

	if err = pc.SetRemoteDescription(signallingAnswer.WebRTCSessionDescription); err != nil {
		requestLogger.Error(
			"error while setting connection local description in dialing",
			"err", err,
			"signallingAnswer", signallingAnswer,
		)
		pc.Close()
		return nil, err
	}
	requestLogger.Info("peer connection set")

	// Wait for ICE to resolve, finalizing connection
	<-webrtc.GatheringCompletePromise(pc)
	requestLogger.Debug("offering peer connection ICE resolved")

	return offeringPeer, nil
}

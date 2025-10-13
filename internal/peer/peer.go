package peer

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

const (
	HEARTBEAT_PERIOD time.Duration = 10 * time.Second
)

// A Peer object should not be constructed directly.
// Instead, Peers should be made using a PeerFactory and the corresponding NewOfferingPeer and NewAnsweringPeer methods.
//
// Even then, these methods are likely never going to be called explicitly by the main client, and
// Peers will instead be constructed by a WebRTCConnectionManager using Dial (returning a Peer directly)
// or listenIncomingSessionOffers (returning a Peer along the connection manager's) IncomingConnectionChannel channel.
type Peer struct {
	logger *slog.Logger
	uuid   uuid.UUID

	// --------------------------------------------------------------------------------
	// Connection related fields

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection

	// WebRTC track for sending audio from this client to the remote client
	connectionAudioInputTrack *webrtc.TrackLocalStaticSample

	// Data Channel to send / receive heartbeat messages on
	connectionHeartbeatDataChannel *webrtc.DataChannel

	// --------------------------------------------------------------------------------
	// Audio Input / Output fields

	// Audio input data from this client is passed in on this channel to be sent to remote peers.
	audioInputChannel <-chan []int16
	// Function to signal the closing of the audioInputChannel, meaning no more data is to be sent along it.
	audioInputChannelCancelFunc context.CancelFunc
	// audioEncoder

	// Audio output data from this client is passed along this channel to be played on the audio output device.
	audioOutputChannel chan<- []int16
	// audioDecoder

}

// Set the audioInputChannel of this peer.
// The given channel should stream raw PCM frames from this clients audio input device (e.g. microphone)/
//
// When this peer is shutdown, the given cancel function is called to signal no more data is to be sent on the channel.
func (peer *Peer) SetAudioInputChannel(c <-chan []int16, cancel context.CancelFunc) {
	peer.audioInputChannel = c
	peer.audioInputChannelCancelFunc = peer.audioInputChannelCancelFunc
}

// Set the audioOutputChannel of this peer.
// The given channel should accept raw PCM frames to be played on this clients audio output device.
// The source of these frames is the audio input device of the remote peer.
//
// When this peer is shutdown, the given channel is closed (hence, no data is to be sent on it anymore)
func (peer *Peer) SetAudioOutputChannel(c chan<- []int16) {
	peer.audioOutputChannel = c
}

func (peer *Peer) setConnectionHeartbeatDataChannel(dc *webrtc.DataChannel) {
	peer.connectionHeartbeatDataChannel = dc
	dc.OnOpen(peer.heartbeatOnOpenHandler)
	dc.OnMessage(peer.heartbeatOnMessageHandler)
}

func (peer *Peer) setConnectionAudioInputTrack(tr *webrtc.TrackLocalStaticSample) {
	peer.connectionAudioInputTrack = tr
}

func (peer *Peer) gracefulShutdown() {
	peer.audioInputChannelCancelFunc()
	close(peer.audioOutputChannel)
	peer.connection.Close()
}

// --------------------------------------------------------------------------------
// CONNECTION HANDLERS
// Handlers for various aspects of the PeerConnection

// OnConnectionStateChange handler
// Handles changes of state on the connection, such as connection establishment and graceful shutdown
func (peer *Peer) onConnectionStateChangeHandler(pcs webrtc.PeerConnectionState) {
	peer.logger.Debug("peer connection state change", "new state", pcs.String())
	switch pcs {
	case webrtc.PeerConnectionStateConnected:
		peer.logger.Info("peer connection connected")
		// TODO: Set encoder/decoder based on negotiated codec
		// TODO: Start streaming audio input data along connectionAudioInputTrack
	case webrtc.PeerConnectionStateDisconnected:
		peer.logger.Info("peer connection disconnected")
		// TODO: Handle disconnected connection
		peer.gracefulShutdown()
	case webrtc.PeerConnectionStateClosed:
		peer.logger.Info("peer connection closed")
		// TODO: Handle closed connection
		peer.gracefulShutdown()
	}
}

// OnTrack handler
// Handle initialization of a new track, offered by the remote peer.
// Should start listening for packets from the remote peer, decoding them, and streaming them out
func (peer *Peer) onTrackHandler(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
	peer.logger.Debug(
		"received track",
		"track ID", tr.ID(),
		"track kind", tr.Kind().String(),
	)
	go func() {
		for {
			_, _, err := tr.ReadRTP()
			if err != nil {
				peer.logger.Error("error while receiving data from remote peer", "err", err)
				continue
			}

			// TODO: Decode packet payload and send along audioOutputChannel
			// peer.audioOutputChannel <- pkt.Payload

			// TODO: Handle dropped and out-of-order packets?
		}
	}()
}

// heartbeat onOpen handler
// Once opened, send a heartbeat message occasionally on the channel
func (peer *Peer) heartbeatOnOpenHandler() {
	heartbeatTicker := time.NewTicker(HEARTBEAT_PERIOD)
	defer heartbeatTicker.Stop()
	for {
		sendingTimestamp := <-heartbeatTicker.C
		msg, err := sendingTimestamp.MarshalBinary()
		if err != nil {
			peer.logger.Error("error while marshalling sending timestamp to binary", "err", err)
			continue
		}
		peer.logger.Debug("sending heartbeat", "sendingTimestamp", sendingTimestamp)
		if err := peer.connectionHeartbeatDataChannel.Send(msg); err != nil {
			slog.Error("error when sending heartbeat", "err", err)
		}
	}
}

// heartbeat onMessage handler
// handle a new message on the heartbeat data channel
func (peer *Peer) heartbeatOnMessageHandler(msg webrtc.DataChannelMessage) {
	currentTime := time.Now()

	var sendingTime time.Time
	sendingTime.UnmarshalBinary(msg.Data)

	networkLatency := currentTime.Sub(sendingTime)

	peer.logger.Debug(
		"received heartbeat",
		"networkLatency", networkLatency,
		"currentTime", currentTime,
		"sendingTime", sendingTime,
	)
}

// audioInputTrack onOpen handler
// Handle audio input along the audioInputDataChannel by forwarding through the PeerConnection audio track
// This method blocks waiting for data on the audioInputDataChannel, so run in a goroutine
func (peer *Peer) sendAudioInputHandler() {
	for _ = range peer.audioInputChannel {
		// TODO: Encode the sample and send along the connectionAudioInputTrack

	}
}

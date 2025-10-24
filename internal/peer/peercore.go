package peer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling"
	"github.com/pion/webrtc/v4"
)

const (
	HEARTBEAT_PERIOD time.Duration = 5 * time.Second
)

// The core of the peer, holding transport layer information such as the PeerConnection.
// A peerCore is created by the PeerFactory when a connection is started, but not yet completed.
// In this sense, a peerCore is the fundamental networking of a peer, but it is not yet
// at the point where an entire Peer can be constructed around that connection.
//
// For example, a peerCore may not have a negotiated codec for VoIP information,
// or a connectionAudioOutputTrack defined, meaning no audio can be sent.
//
// When the associated connection is moved to a connected state, the peerCoreConnectionStateChangeHandler
// (which is attached by the PeerFactory, see NewOfferingPeer, NewAnsweringPeer methods)
// the PeerFactory finishes the Peer setup by creating an OPUS Encoder, sets audio channels, and
// embeds the peerCore in a Peer struct to be returned to the application.
type peerCore struct {
	logger *slog.Logger

	// The Identifier of the *remote* client, i.e. the identifier of the client this peer represents
	identifier signalling.PeerIdentifier

	// This context handles signalling to handlers that the peer is shutting down
	// Methods may listen for closing (calling the ctxCancelFunction), with <-ctx.Done()
	ctx           context.Context
	ctxCancelFunc context.CancelFunc

	shutdownOnce sync.Once

	// --------------------------------------------------------------------------------
	// Connection related fields

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection

	// WebRTC track for sending audio from this client to the remote client.
	// This parameter is undefined until the connection has been negotiated
	connectionAudioInputTrack *webrtc.TrackLocalStaticSample

	// WebRTC track for receiving audio from remote client.
	// This parameter is undefined until the connection has been negotiated
	connectionAudioOutputTrack *webrtc.TrackRemote

	// Data Channel to send / receive heartbeat messages on.
	// This parameter is undefined until the connection has been negotiated
	connectionHeartbeatDataChannel *webrtc.DataChannel
}

func newPeerCore(
	identifier signalling.PeerIdentifier,
	connection *webrtc.PeerConnection,
) *peerCore {
	ctx, cancelFunc := context.WithCancel(context.Background())
	core := &peerCore{
		identifier:    identifier,
		connection:    connection,
		ctx:           ctx,
		ctxCancelFunc: cancelFunc,
	}

	core.logger = slog.Default().With(
		"peer uuid", core.identifier.Uuid,
	)

	core.connection.OnTrack(core.onTrackHandler)
	core.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch dc.Label() {
		case "heartbeat":
			core.setConnectionHeartbeatDataChannel(dc)
		}
	})

	return core
}

// --------------------------------------------------------------------------------
// PUBLIC METHODS
// These methods are used by both peerCore and Peer

// Get the context of this peer
// May be used to determine if the peer is shutting down by listening for <-ctx.Done()
func (core *peerCore) GetContext() context.Context {
	return core.ctx
}

func (core *peerCore) Identifier() signalling.PeerIdentifier {
	return core.identifier
}

// This method is shadowed by Peer, and hence needs to only handle shutdown of
// the core methods
func (core *peerCore) Close() {
	core.shutdownOnce.Do(func() {
		core.ctxCancelFunc()
		core.connection.Close()
	})
}

// --------------------------------------------------------------------------------
// PRIVATE UTIL METHODS

func (core *peerCore) setConnectionHeartbeatDataChannel(dc *webrtc.DataChannel) {
	core.connectionHeartbeatDataChannel = dc
	dc.OnOpen(core.heartbeatOnOpenHandler)
	dc.OnMessage(core.heartbeatOnMessageHandler)
}

func (core *peerCore) setConnectionAudioInputTrack(tr *webrtc.TrackLocalStaticSample) {
	core.connectionAudioInputTrack = tr
}

// --------------------------------------------------------------------------------
// CONNECTION HANDLERS
// Handlers for various aspects of the PeerConnection

// OnTrack handler
// Handle initialization of a new track, offered by the remote peer.
// Should start listening for packets from the remote peer, decoding them, and streaming them out
func (core *peerCore) onTrackHandler(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
	core.logger.Debug(
		"received track",
		"track ID", tr.ID(),
		"track kind", tr.Kind().String(),
	)

	core.connectionAudioOutputTrack = tr
}

// heartbeat onOpen handler
// Once opened, send a heartbeat message occasionally on the channel
func (core *peerCore) heartbeatOnOpenHandler() {
	heartbeatTicker := time.NewTicker(HEARTBEAT_PERIOD)
	defer heartbeatTicker.Stop()
	for {
		sendingTimestamp := <-heartbeatTicker.C
		select {
		case <-core.ctx.Done():
			return
		default:
		}

		msg, err := sendingTimestamp.MarshalBinary()
		if err != nil {
			core.logger.Error("error while marshalling sending timestamp to binary", "err", err)
			continue
		}
		core.logger.Debug("sending heartbeat", "sendingTimestamp", sendingTimestamp)
		if err := core.connectionHeartbeatDataChannel.Send(msg); err != nil {
			slog.Error("error when sending heartbeat", "err", err)
		}
	}
}

// heartbeat onMessage handler
// handle a new message on the heartbeat data channel
func (core *peerCore) heartbeatOnMessageHandler(msg webrtc.DataChannelMessage) {
	currentTime := time.Now()

	var sendingTime time.Time
	sendingTime.UnmarshalBinary(msg.Data)

	networkLatency := currentTime.Sub(sendingTime)

	core.logger.Debug(
		"received heartbeat",
		"networkLatency", networkLatency,
		"currentTime", currentTime,
		"sendingTime", sendingTime,
	)
}

package peer

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type PeerFactory struct {
	logger *slog.Logger
}

func NewPeerFactory(logger *slog.Logger) *PeerFactory {
	if logger == nil {
		logger = slog.Default()
	}

	factory := &PeerFactory{
		logger: logger,
	}

	return factory
}

// Handle connection set up common to both offering and answering clients
func (factory PeerFactory) commonConnectionSetup(peer *Peer) {
	peer.connection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		peer.logger.Debug("peer connection state change", "new state", pcs.String())
		switch pcs {
		case webrtc.PeerConnectionStateConnected:
			peer.logger.Info("peer connection connected")
		case webrtc.PeerConnectionStateDisconnected:
			peer.logger.Info("peer connection disconnected")
			// TODO: Handle disconnected connection
			// Should close peer.connection, somehow signal this peer is to be discarded
		case webrtc.PeerConnectionStateClosed:
			peer.logger.Info("peer connection closed")
			// TODO: Handle closed connection
			// Same as disconnected, close peer.connection, signal discard
		}
	})
}

// Handle creation of a new peer on the offering side of the connection.
//
// Takes a created (but not processed) *webrtc.PeerConnection, and adds
// heartbeat and outgoing audio track.
//
// If anything goes wrong, this method returns a nil Peer and a non-nil error.
func (factory PeerFactory) NewOfferingPeer(connection *webrtc.PeerConnection) (*Peer, error) {

	peer := &Peer{
		uuid:       uuid.New(),
		connection: connection,
	}
	peer.logger = slog.Default().With(
		"peer uuid", peer.uuid,
	)

	// TODO: Complete creation of datachannel before offer is made
	heartbeatDataChannel, err := connection.CreateDataChannel("heartbeat", &webrtc.DataChannelInit{})
	if err != nil {
		peer.logger.Error("error while creating heartbeat channel", "err", err)
		return nil, err
	}
	heartbeatDataChannel.OnOpen(func() { peer.heartbeatSendMessageHandler(heartbeatDataChannel) })
	heartbeatDataChannel.OnMessage(peer.heartbeatOnMessageHandler)

	factory.commonConnectionSetup(peer)

	return peer, nil
}

// Handle creation of a new peer on the answering side of the connection.
//
// Takes a created (but not processed) *webrtc.PeerConnection, and adds
// an outgoing audio track. The heartbeat channel is made by the offering peer.
//
// If anything goes wrong, this method returns a nil Peer and a non-nil error.
func (factory PeerFactory) NewAnsweringPeer(connection *webrtc.PeerConnection) (*Peer, error) {
	peer := &Peer{
		uuid:       uuid.New(),
		connection: connection,
	}
	peer.logger = slog.Default().With(
		"peer uuid", peer.uuid,
	)

	peer.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch dc.Label() {
		case "heartbeat":
			dc.OnOpen(func() { peer.heartbeatSendMessageHandler(dc) })
			dc.OnMessage(peer.heartbeatOnMessageHandler)
		}
	})

	factory.commonConnectionSetup(peer)

	return peer, nil
}

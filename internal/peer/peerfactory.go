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



	return peer, nil
}

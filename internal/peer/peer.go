package peer

import (
	"log/slog"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type Peer struct {
	logger *slog.Logger

	uuid uuid.UUID

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection
}

package peer

import "github.com/pion/webrtc/v4"

type Peer struct {
	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection
}

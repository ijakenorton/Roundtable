package networking

import "github.com/pion/webrtc/v4"

// Holds all relevant information for the signalling server to route
// a request from offerer to answerer across the network
// in order to facilitate a new p2p connection between the two.
type SignallingOffer struct {
	// The public IP address of the peer, including port number.
	// Should be discovered by the
	RemoteEndpoint string

	WebRTCSessionDescription webrtc.SessionDescription
}

package application

// The main application representation for the client.
//
// Holds references to the audio input / output devices,
// the audio IO library (e.g. RTAudio, PortAudio, to generate above devices),
// the connected peers, and so on.
//
// This struct provides a good basis for integration of the TUI.
type App struct {
	ConnectedPeers []*ApplicationPeer
}

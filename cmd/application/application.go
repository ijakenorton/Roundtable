package application

import (
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/networking"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling"
)

// The main application representation for the client.
//
// Holds references to the audio input / output devices,
// the audio IO library (e.g. RTAudio, PortAudio, to generate above devices),
// the connected peers, and so on.
//
// This struct provides a good basis for integration of the TUI.
type App struct {
	// --------------------------------------------------------------------------------
	// Connections and Peers

	// Handle connections (offering and answering) and produce connected peers on
	// the ConnectedPeerChannel.
	connectionManager *networking.ConnectionManager

	// A list of currently connected peers. All peers should receive data from
	// the client's audio input device and send data to the client's output device.
	//
	// These connections should be made when the Peer is received from the ConnectionManager.
	connectedPeers []*ApplicationPeer

	// A list of all rejected peers, those that have been disconnected
	// or failed to connected during the lifetime of this client
	// (e.g. by a Codec mismatch). Use this information to prevent
	// trying to reconnect to the same failing peers over and over.
	//
	// The PeerIdentifier.UUID is unique to an instance of Roundtable,
	// so a remote client restarting will generate a new UUID
	// and hence allow a new attempt at connection.
	rejectedPeerIdentifiers []signalling.PeerIdentifier

	// --------------------------------------------------------------------------------
	// Audio Input, Output, API, and Devices

	// The audio device API for the host machine. Allows querying of
	// input and output devices (microphones and speakers) and for
	// opening / selecting those input / output devices.
	//
	// Supported APIs are RTAudio and PortAudio
	// TODO: Implement this struct logic via interface

	// Audio Data Flow from Application to Peer (input path)
	// | ----------------------------------- Application ----------------------------------- |	   | -------- ApplicationPeer -------- |
	// Client's audio input device (e.g. microphone) -> AudioAugmentationDevice -> FanOutDevice -> [AudioFormatConversionDevice -> Peer]

	// The audio input device of the client, i.e. the microphone of choice
	audioInputDevice audiodevice.AudioSourceDevice

	// Augmentation of the input audio, e.g. for setting this client's volume before sending to the remote peer
	inputAugmentationDevice *device.AudioAugmentationDevice

	// FanOutDevice to copy audio data from the microphone (more specifically the inputAugmentationDevice) to all connected peers
	inputFanOutDevice *device.FanOutDevice

	// Audio Data Flow from Peer to Application (output path)
	// | ---------------------- ApplicationPeer ---------------------- |    | -------------------- Application -------------------- |
	// [ Peer -> AudioFormatConversionDevice -> AudioAugmentationDevice] -> FanInDevice -> Client's audio output device (e.g. speaker)

	// The audio output device, i.e. the speaker of choice
	audioOutputDevice audiodevice.AudioSinkDevice

	// FanInDevice to mix audio from all connected peers back into a single frame to send to speakers
	outputFanInDevice *device.FanInDevice
}

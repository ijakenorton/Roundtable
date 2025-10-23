package application

import (
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/peer"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling"
)

// A wrapper around a Peer (github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/peer/peer.go)
// to encapsulate a variety of peer-specific items.
//
// The Peer itself is included, meaning the codec and many networking elements are included,
// as is the PeerIdentifier (github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling/peeridentifier.go)
// so is the AudioFormatConversionDevice (github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device/audioformatconversiondevice.go)
// and so is the AudioAugmentationDevice (github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device/audioaugmentationdevice.go)
//
// Effectively, this struct is a nice interface for sending/receiving audio from the network at an application-level without
// worrying about the underlying devices.
//
// When modelling this struct as a AudioSourceDevice, the PCMFrames are sourced from the AudioAugmentationDevice,
// (which comes after the AudioFormatConversionDevice) and not the Peer directly.
// This means that, assuming everything has been constructed correctly, the PCMFrames sourced are in the same format as the
// client's AudioOutputDevice.
// It is *not* enforced that the audio arrives with a specific frame duration, e.g. one ApplicationPeer might source frames
// of 20ms, another frames of 40ms, and another still with frames of 2.5ms. This discrepancy is inherent to the audio transmission
// of peers across the network and must be handled by a downstream device.
// It is expected that the SourceStream of this struct is added to a FanInDevice to mix the audio from multiple peers at once.
// The FanInDevice will also handle the above issue of frame durations by buffering the sourced frames and producing
// evenly sampled lengths from each input.
// | ---------------------- ApplicationPeer ---------------------- |
// [ Peer -> AudioFormatConversionDevice -> AudioAugmentationDevice] -> FanInDevice -> Client's audio output device (e.g. speaker)
//
// When modelling this struct as a AudioSinkDevice, the PCMFrames are fed into an AudioAugmentationDevice,
// (which before after the AudioFormatConversionDevice) and not the Peer directly.
// This means that the sink frames are expected to be in the format of the client's audio input device, i.e.
// frames may have a different sample rate, different number of channels, or a different frame duration than
// what is expected by the underlying peer, but this is handled by the audioFormatConversionDevice.
// The SinkStream of this peer should be added to a FanOutDevice to allow the client's audio input device to be passed to multiple peers.
// \ 																| ---------------------- ApplicationPeer ---------------------- |
// Client's audio input device (e.g. microphone) -> FanOutDevice -> [ AudioAugmentationDevice -> AudioFormatConversionDevice -> Peer]
//
// A new ApplicationPeer should be constructed by listening to the ConnectionManager's ConnectedPeerChannel for newly connected peers
// (github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/networking/connectionmanager.go)
type ApplicationPeer struct {
	peer *peer.Peer

	// The ID of this item is set to be the ID of the wrapper Peer
	peerID signalling.PeerIdentifier

	audioAugmentationDevice     *device.AudioAugmentationDevice
	audioFormatConversionDevice *device.AudioFormatConversionDevice
}

// Get the peer identifier for this peer.
// Useful for displaying something unique about this peer, as the included UUID is
// unique to the connection.
func (p ApplicationPeer) GetPeerIdentifier() signalling.PeerIdentifier {
	return p.peerID
}

// Get the device properties for the sourced PCMFrames, i.e. the properties of the device *after* conversion
// through the AudioFormatConversionDevice
func (p ApplicationPeer) GetDeviceProperties() audiodevice.DeviceProperties {
	return p.audioAugmentationDevice.GetDeviceProperties()
}

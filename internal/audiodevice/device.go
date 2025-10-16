package audiodevice

import "github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/frame"

type DeviceProperties struct {
	SampleRate  int
	NumChannels int
}

// Interface for audio source device, e.g. microphones
//
// Source devices need only define some way to get data out of the device,
// which returns a channel (stream) of PCMFrames
type AudioSourceDevice interface {
	// Get the stream of this audio device.
	//
	// Raw audio data (as PCMFrames) will arrive on the returned channel.
	GetStream() <-chan frame.PCMFrame

	// Meaningfully close the AudioSourceDevice, including any cleanup of
	// memory and closing of channels.
	//
	// It is assumed that once closed, this device will transmit no more information.
	Close()

	GetDeviceProperties() DeviceProperties
}

// Interface for audio sink devices, e.g. speakers
//
// Sink devices need only define some way to consume data,
// taken as a channel (stream) of audio.PCMFrames
type AudioSinkDevice interface {
	// Set the source stream of this audio device.
	//
	// Raw audio data (as PCMFrames) will arrive on the given channel.
	//
	// When this stream is closed, it is assumed the device will be cleaned up
	// (memory will be freed, other channels will be closed, etc)
	SetStream(sourceStream <-chan frame.PCMFrame)

	GetDeviceProperties() DeviceProperties
}

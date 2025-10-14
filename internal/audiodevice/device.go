package audiodevice

import "github.com/hmcalister/roundtable/internal/frame"

type DeviceProperties struct {
	SampleRate  int
	NumChannels int
}

// Interface for audio input devices, e.g. microphones
//
// Input devices need only define some way to get data out of the device,
// which returns a channel (stream) of PCMFrames
type AudioInputDevice interface {
	// Get the input stream of this audio device.
	//
	// Raw audio data (as PCMFrames) will arrive on the returned channel.
	GetStream() <-chan frame.PCMFrame

	// Meaningfully close the AudioInputDevice, including any cleanup of
	// memory and closing of channels.
	//
	// It is assumed that once closed, this device will transmit no more information.
	Close()

	GetDeviceProperties() DeviceProperties
}

// Interface for audio output devices, e.g. speakers
//
// Output devices need only define some way to consume data,
// taken as a channel (stream) of audio.PCMFrames
type AudioOutputDevice interface {
	// Set the data stream of this audio device.
	//
	// Raw audio data (as PCMFrames) will arrive on the given channel
	// and should be passed meaningfully be the output device (e.g. sent to speakers).
	//
	// When this stream is closed, it is assumed the device will be cleaned up
	// (memory will be freed, other channels will be closed, etc)
	SetStream(incomingAudioStream <-chan frame.PCMFrame)

	GetDeviceProperties() DeviceProperties
}

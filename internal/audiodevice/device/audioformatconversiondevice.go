package device

import (
	"sync"

	"github.com/hmcalister/roundtable/internal/audiodevice"
	"github.com/hmcalister/roundtable/internal/frame"
)

// Middle-man processing device to handle format mismatches
// between the source data format to the sink data format.
//
// e.g. if the source format is mono, but the sink format specifies stereo,
// this device will handle the conversion.
//
// This device is both a sink and a source!
type AudioFormatConversionDevice struct {
	// For this device only, the naming convention for the channels is very confusing.
	// We take the convention that the source channel is the *external* source,
	// i.e. the channel data arrives on.
	//
	// Likewise, the sink channel is the *external* sink, i.e.
	// the channel data leaves on.
	//
	// This means the naming convention is backwards to what is expected.
	// GetStream returns the sink channel.
	// SetStream sets the source channel.

	// The stream that data *arrives on*
	sourceChannel    <-chan frame.PCMFrame
	sourceProperties audiodevice.DeviceProperties

	// The stream that data *leaves on*
	sinkChannel    chan frame.PCMFrame
	sinkProperties audiodevice.DeviceProperties

	// The functions to apply when processing the source data to sink format
	formatConversionFunctions []audioFormatConversionFunction

	shutdownOnce sync.Once
}

// Create a new AudioFormatConversionDevice by defining:
// - the source properties (the properties of the audio being fed into this device)
// - the sink properties (the properties of the audio leaving this device)
//
// Note one must still call SetStream, passing in the source channel,
// and GetStream, to receive the sink channel, to use this device, in an
// effort to remain consistent with the device interfaces.
//
// This device will only start converting once SetStream is called.
func NewProcessingStream(
	sourceProperties audiodevice.DeviceProperties,
	sinkProperties audiodevice.DeviceProperties,
) (AudioFormatConversionDevice, error) {
	formatConversionFunctions := make([]audioFormatConversionFunction, 0)

	if sourceProperties.NumChannels == 1 && sinkProperties.NumChannels == 2 {
		formatConversionFunctions = append(formatConversionFunctions, monoToStereo)
	}
	if sourceProperties.NumChannels == 2 && sinkProperties.NumChannels == 1 {
		formatConversionFunctions = append(formatConversionFunctions, stereoToMono)
	}
	if sourceProperties.SampleRate != sinkProperties.SampleRate {
		formatConversionFunctions = append(formatConversionFunctions, newResampleFunction(sinkProperties.SampleRate))
	}

	return AudioFormatConversionDevice{
		sourceProperties:          sourceProperties,
		sinkProperties:            sinkProperties,
		sinkChannel:               make(chan frame.PCMFrame),
		formatConversionFunctions: formatConversionFunctions,
	}, nil
}

// --------------------------------------------------------------------------------
// AudioSourceDevice Interface

// Get the source stream of this audio device.
// Raw audio data (as PCMFrames) will arrive on the returned channel.
func (d *AudioFormatConversionDevice) GetStream() <-chan frame.PCMFrame {
	return d.sinkChannel
}

// Meaningfully close the AudioSourceDevice, including any cleanup of
// memory and closing of channels.
//
// It is assumed that once closed, this device will transmit no more information.
func (d *AudioFormatConversionDevice) Close() {
	d.shutdownOnce.Do(func() {
		close(d.sinkChannel)
	})
}

// WARNING:
// GetDeviceProperties of the AudioFormatConversionDevice returns the
// device properties of the LEAVING data. i.e. the data that exits this device!
//
// If you need the properties of the data entering this device, call GetSourceDeviceProperties()
func (d *AudioFormatConversionDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return d.sinkProperties
}

// --------------------------------------------------------------------------------
// AudioSinkDevice Interface

// Set the source channel of this audio device, i.e. where data comes from.
// Raw audio data (as PCMFrames) will arrive on the given channel.
//
// When this stream is closed, it is assumed the device will be cleaned up
// (memory will be freed, other channels will be closed, etc)
func (d *AudioFormatConversionDevice) SetStream(sourceChannel <-chan frame.PCMFrame) {
	d.sourceChannel = sourceChannel
	go func() {
		for pcmFrame := range d.sourceChannel {
			for _, f := range d.formatConversionFunctions {
				pcmFrame = f(pcmFrame)
			}
			d.sinkChannel <- pcmFrame
		}
		// This goroutine dies when incomingAudioStream is closed.
	}()
}

func (d *AudioFormatConversionDevice) GetSourceDeviceProperties() audiodevice.DeviceProperties {
	return d.sourceProperties
}

// --------------------------------------------------------------------------------

type audioFormatConversionFunction func(frame.PCMFrame) frame.PCMFrame

func monoToStereo(sourceFrame frame.PCMFrame) frame.PCMFrame {
	// TODO
	return sourceFrame
}

func stereoToMono(sourceFrame frame.PCMFrame) frame.PCMFrame {
	// TODO
	return sourceFrame
}

func newResampleFunction(newSampleRate int) audioFormatConversionFunction {
	return func(sourceFrame frame.PCMFrame) frame.PCMFrame {
		// TODO
		return sourceFrame
	}
}

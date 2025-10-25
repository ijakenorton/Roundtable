package device

import (
	"sync"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
)

// An AudioSourceDevice that will never produce a frame.
//
// A minimal example of the architecture of an AudioSourceDevice, useful in testing.
type DummyAudioSourceDevice struct {
	properties   audiodevice.DeviceProperties
	shutdownOnce sync.Once
	sinkStream   chan frame.PCMFrame
}

func NewDummyAudioSourceDevice(properties audiodevice.DeviceProperties) *DummyAudioSourceDevice {
	return &DummyAudioSourceDevice{
		properties: properties,
		sinkStream: make(chan frame.PCMFrame),
	}
}

func (d *DummyAudioSourceDevice) Close() {
	d.shutdownOnce.Do(func() {
		close(d.sinkStream)
	})
}

func (d *DummyAudioSourceDevice) GetStream() <-chan frame.PCMFrame {
	return d.sinkStream
}

func (d *DummyAudioSourceDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return d.properties
}

// An AudioSinkDevice that consumes all frames without any further actions.
//
// A minimal example of the architecture of an AudioSinkDevice, useful in testing.
type DummyAudioSinkDevice struct {
	properties   audiodevice.DeviceProperties
	sourceStream <-chan frame.PCMFrame
}

func NewDummyAudioSinkDevice(properties audiodevice.DeviceProperties) *DummyAudioSinkDevice {
	return &DummyAudioSinkDevice{
		properties: properties,
	}
}

func (d *DummyAudioSinkDevice) SetStream(sourceStream <-chan frame.PCMFrame) {
	d.sourceStream = sourceStream
	go func() {
		for range sourceStream {
		}
	}()
}

func (d DummyAudioSinkDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return d.properties
}

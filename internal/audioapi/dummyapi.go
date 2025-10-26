package audioapi

import (
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
)

// A dummy API that lists only one input and one output device:
// - a dummy input device (produces no frames, ever)
// - a dummy output device (consumes all frames and does nothing)
//
// This API is intended to be used in testing only!
type DummyAudioIODeviceAPI struct {
	properties audiodevice.DeviceProperties
}

func NewDummyAudioIODeviceAPI(properties audiodevice.DeviceProperties) DummyAudioIODeviceAPI {
	return DummyAudioIODeviceAPI{
		properties: properties,
	}
}

func (api DummyAudioIODeviceAPI) InputDevices() []AudioIODevice {
	return []AudioIODevice{
		{
			ID:               0,
			Name:             "DummyInput",
			DeviceProperties: api.properties,
		},
	}
}

func (api DummyAudioIODeviceAPI) InitInputDeviceFromID(id AudioIODevice) (audiodevice.AudioSourceDevice, error) {
	if id.ID != 0 {
		return nil, errNoDeviceWithID
	}
	return device.NewDummyAudioSourceDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) InitDefaultInputDevice() (audiodevice.AudioSourceDevice, error) {
	return device.NewDummyAudioSourceDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) OutputDevices() []AudioIODevice {
	return []AudioIODevice{
		{
			ID:               0,
			Name:             "DummyOutput",
			DeviceProperties: api.properties,
		},
	}
}

func (api DummyAudioIODeviceAPI) InitOutputDeviceFromID(id AudioIODevice) (audiodevice.AudioSinkDevice, error) {
	if id.ID != 0 {
		return nil, errNoDeviceWithID
	}
	return device.NewDummyAudioSinkDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) InitDefaultOutputDevice() (audiodevice.AudioSinkDevice, error) {
	return device.NewDummyAudioSinkDevice(api.properties), nil
}

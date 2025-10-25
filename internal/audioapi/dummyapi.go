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

func (api DummyAudioIODeviceAPI) GetAllInputDevices() []AudioIODeviceID {
	return []AudioIODeviceID{
		{
			ID:               0,
			Name:             "DummyInput",
			DeviceProperties: api.properties,
		},
	}
}

func (api DummyAudioIODeviceAPI) GetInputDevice(id AudioIODeviceID) (audiodevice.AudioSourceDevice, error) {
	if id.ID != 0 {
		return nil, errNoDeviceWithID
	}
	return device.NewDummyAudioSourceDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) GetDefaultInputDevice() (audiodevice.AudioSourceDevice, error) {
	return device.NewDummyAudioSourceDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) GetAllOutputDevices() []AudioIODeviceID {
	return []AudioIODeviceID{
		{
			ID:               0,
			Name:             "DummyOutput",
			DeviceProperties: api.properties,
		},
	}
}

func (api DummyAudioIODeviceAPI) GetOutputDevice(id AudioIODeviceID) (audiodevice.AudioSinkDevice, error) {
	if id.ID != 0 {
		return nil, errNoDeviceWithID
	}
	return device.NewDummyAudioSinkDevice(api.properties), nil
}

func (api DummyAudioIODeviceAPI) GetDefaultOutputDevice() (audiodevice.AudioSinkDevice, error) {
	return device.NewDummyAudioSinkDevice(api.properties), nil
}

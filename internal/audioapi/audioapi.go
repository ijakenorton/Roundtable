package audioapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
)

var (
	errNoDefaultDevice = errors.New("no default device available")
	errNoDeviceWithID  = errors.New("no device with specified ID")
)

type AudioIODevice struct {
	// The ID of the device
	//
	// Should come from the underlying API (RTAudio/PortAudio),
	// But could be defined in some programmatic way by the AudioIODeviceAPI
	//
	// Intended to be the canonical way to reference the AudioIODevice
	// (e.g. a microphone or speaker), such that when telling the API
	// to use a device as the default input/output, it is this value
	// that is used to identify the device.
	ID int

	// A human-readable name for the device, if one exists.
	// Not necessary, and not canonical.
	Name string

	// The device properties (sample rate and channels) of this device.
	// Note that Roundtable only supports devices with mono or stereo channels.
	DeviceProperties audiodevice.DeviceProperties
}

func (device AudioIODevice) String() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "ID:          %d\n", device.ID)
	fmt.Fprintf(&sb, "Name:        %s\n", device.Name)
	fmt.Fprintf(&sb, "SampleRate:  %d\n", device.DeviceProperties.SampleRate)
	fmt.Fprintf(&sb, "NumChannels: %d\n", device.DeviceProperties.NumChannels)
	return sb.String()
}

// Define an API to interface with hardware devices.
// Intended to be an abstract way to:
// - Query existing devices (input and output)
// - Initialize an input/output device as an AudioSourceDevice/AudioSinkDevice respectively
//
// Implementations could include small wrappers around RTAudio and PortAudio
type AudioIODeviceAPI interface {
	InputDevices() []AudioIODevice
	InitInputDeviceFromID(AudioIODevice) (audiodevice.AudioSourceDevice, error)
	InitDefaultInputDevice() (audiodevice.AudioSourceDevice, error)

	OutputDevices() []AudioIODevice
	InitOutputDeviceFromID(AudioIODevice) (audiodevice.AudioSinkDevice, error)
	InitDefaultOutputDevice() (audiodevice.AudioSinkDevice, error)
}

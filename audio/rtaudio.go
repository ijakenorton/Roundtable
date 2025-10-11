package audio

/*
#cgo linux CFLAGS: -D__UNIX_JACK__
#cgo linux LDFLAGS: -lpthread -lm -ljack -lstdc++
#cgo windows CFLAGS: -D__WINDOWS_WASAPI__
#cgo windows LDFLAGS: -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid -lstdc++

#define AUDIO_WRAPPER_IMPLEMENTATION
#include "audio_wrapper.h"
#include "rtaudio_c.cpp"
#include "RtAudio.cpp"
*/
import "C"
import (
	"errors"
	"unsafe"
)

// AudioDevice wraps the C audio device handle
type AudioDevice struct {
	handle *C.audio_device_t
}

// Create creates a new audio device instance
func Create() (*AudioDevice, error) {
	handle := C.audio_create()
	if handle == nil {
		return nil, errors.New("failed to create audio device")
	}
	return &AudioDevice{handle: handle}, nil
}

// Destroy frees the audio device and all resources
func (a *AudioDevice) Destroy() {
	if a.handle != nil {
		C.audio_destroy(a.handle)
		a.handle = nil
	}
}

// DeviceCount returns the number of available audio devices
func (a *AudioDevice) DeviceCount() int {
	return int(C.audio_device_count(a.handle))
}

// GetDefaultInput returns the default input device ID
func (a *AudioDevice) GetDefaultInput() uint {
	return uint(C.audio_get_default_input(a.handle))
}

// GetDefaultOutput returns the default output device ID
func (a *AudioDevice) GetDefaultOutput() uint {
	return uint(C.audio_get_default_output(a.handle))
}

// StartRecording starts recording from the specified device
// Returns the actual buffer size used, or an error
func (a *AudioDevice) StartRecording(deviceID, channels, sampleRate, bufferFrames uint) (int, error) {
	result := C.audio_start_recording(a.handle,
		C.uint(deviceID),
		C.uint(channels),
		C.uint(sampleRate),
		C.uint(bufferFrames))

	if result < 0 {
		errMsg := C.audio_error_message(a.handle)
		return 0, errors.New(C.GoString(errMsg))
	}

	return int(result), nil
}

// Stop stops recording or playback
func (a *AudioDevice) Stop() {
	C.audio_stop(a.handle)
}

// IsRunning returns true if the audio stream is running
func (a *AudioDevice) IsRunning() bool {
	return C.audio_is_running(a.handle) != 0
}

// ReadSamples reads available audio samples (non-blocking)
// buffer should be sized for maxFrames * channels
// Returns the number of frames actually read
func (a *AudioDevice) ReadSamples(buffer []int16, maxFrames uint) (int, error) {
	if len(buffer) == 0 {
		return 0, errors.New("buffer is empty")
	}

	result := C.audio_read_samples(a.handle,
		(*C.int16_t)(unsafe.Pointer(&buffer[0])),
		C.uint(maxFrames))

	if result < 0 {
		errMsg := C.audio_error_message(a.handle)
		return 0, errors.New(C.GoString(errMsg))
	}

	return int(result), nil
}

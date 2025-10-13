package audio

//go:generate go run build.go

/*
#cgo CXXFLAGS: -std=c++11 -g
#cgo CFLAGS: -g
#cgo LDFLAGS: ${SRCDIR}/rtaudio_go.o -lstdc++ -g
#cgo windows CXXFLAGS: -D__WINDOWS_WASAPI__
#cgo windows CFLAGS: -D__WINDOWS_WASAPI__
#cgo windows LDFLAGS: -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid -static
#include "lib/rtaudio_c.h"
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef int16_t sample_t;
typedef struct {
    sample_t *buffer;
    unsigned long bufferBytes;
    unsigned long totalFrames;
    unsigned long frameCounter;
    unsigned int channels;
} InputData;

extern int input_callback(void *outputBuffer, void *inputBuffer,
                   unsigned int nBufferFrames, double streamTime,
                   rtaudio_stream_status_t status, void *userData) {
    InputData *data = (InputData *)userData;

    // Calculate how many frames to copy
    unsigned int frames = nBufferFrames;
    if (data->frameCounter + nBufferFrames > data->totalFrames) {
        frames = data->totalFrames - data->frameCounter;
        data->bufferBytes = frames * data->channels * sizeof(sample_t);
    }

    // Copy data to our buffer
    unsigned long offset = data->frameCounter * data->channels;
    memcpy(data->buffer + offset, inputBuffer, data->bufferBytes);
    data->frameCounter += frames;

    // Return 2 to stop the stream when done
    if (data->frameCounter >= data->totalFrames)
        return 2;
    return 0;
}
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
	"unsafe"
)
func usage() {
   fmt.Printf("\nusage: record N fs <duration> <device> <channelOffset>\n");
   fmt.Printf("    N          = number of channels\n");
   fmt.Printf("    fs         = sample rate\n");
   fmt.Printf("    duration   = optional time in seconds (default = 2.0)\n");
   fmt.Printf("    device     = optional device index (default = 0)\n");
   fmt.Printf("    channelOffset = optional channel offset (default = 0)\n\n");
   os.Exit(1);
}

type API C.rtaudio_api_t
const (
	// APIUnspecified looks for a working compiled API.
	APIUnspecified API = C.RTAUDIO_API_UNSPECIFIED
	// APILinuxALSA uses the Advanced Linux Sound Architecture API.
	APILinuxALSA = C.RTAUDIO_API_LINUX_ALSA
	// APILinuxPulse uses the Linux PulseAudio API.
	APILinuxPulse = C.RTAUDIO_API_LINUX_PULSE
	// APILinuxOSS uses the Linux Open Sound System API.
	APILinuxOSS = C.RTAUDIO_API_LINUX_OSS
	// APIUnixJack uses the Jack Low-Latency Audio Server API.
	APIUnixJack = C.RTAUDIO_API_UNIX_JACK
	// APIMacOSXCore uses Macintosh OS-X Core Audio API.
	APIMacOSXCore = C.RTAUDIO_API_MACOSX_CORE
	// APIWindowsWASAPI uses the Microsoft WASAPI API.
	APIWindowsWASAPI = C.RTAUDIO_API_WINDOWS_WASAPI
	// APIWindowsASIO uses the Steinberg Audio Stream I/O API.
	APIWindowsASIO = C.RTAUDIO_API_WINDOWS_ASIO
	// APIWindowsDS uses the Microsoft Direct Sound API.
	APIWindowsDS = C.RTAUDIO_API_WINDOWS_DS
	// APIDummy is a compilable but non-functional API.
	APIDummy = C.RTAUDIO_API_DUMMY
)

type RtaudioError C.rtaudio_error_t
const (
	RTAUDIO_ERROR_NONE RtaudioError =   C.RTAUDIO_ERROR_NONE          /*!< No error. */
	RTAUDIO_ERROR_WARNING           =	C.RTAUDIO_ERROR_WARNING           /*!< A non-critical error. */
	RTAUDIO_ERROR_UNKNOWN           =	C.RTAUDIO_ERROR_UNKNOWN           /*!< An unspecified error type. */
	RTAUDIO_ERROR_NO_DEVICES_FOUND  =	C.RTAUDIO_ERROR_NO_DEVICES_FOUND  /*!< No devices found on system. */
	RTAUDIO_ERROR_INVALID_DEVICE    =	C.RTAUDIO_ERROR_INVALID_DEVICE    /*!< An invalid device ID was specified. */
	RTAUDIO_ERROR_DEVICE_DISCONNECT =	C.RTAUDIO_ERROR_DEVICE_DISCONNECT /*!< A device in use was disconnected. */
	RTAUDIO_ERROR_MEMORY_ERROR      =	C.RTAUDIO_ERROR_MEMORY_ERROR      /*!< An error occurred during memory allocation. */
	RTAUDIO_ERROR_INVALID_PARAMETER =	C.RTAUDIO_ERROR_INVALID_PARAMETER /*!< An invalid parameter was specified to a function. */
	RTAUDIO_ERROR_INVALID_USE       =	C.RTAUDIO_ERROR_INVALID_USE       /*!< The function was called incorrectly. */
	RTAUDIO_ERROR_DRIVER_ERROR      =	C.RTAUDIO_ERROR_DRIVER_ERROR      /*!< A system driver error occurred. */
	RTAUDIO_ERROR_SYSTEM_ERROR      =	C.RTAUDIO_ERROR_SYSTEM_ERROR      /*!< A system error occurred. */
	RTAUDIO_ERROR_THREAD_ERROR      =	C.RTAUDIO_ERROR_THREAD_ERROR      /*!< A thread error occurred. */
)

func (api API) String() string {
	switch api {
	case APIUnspecified:
		return "unspecified"
	case APILinuxALSA:
		return "alsa"
	case APILinuxPulse:
		return "pulse"
	case APILinuxOSS:
		return "oss"
	case APIUnixJack:
		return "jack"
	case APIMacOSXCore:
		return "coreaudio"
	case APIWindowsWASAPI:
		return "wasapi"
	case APIWindowsASIO:
		return "asio"
	case APIWindowsDS:
		return "directsound"
	case APIDummy:
		return "dummy"
	}
	return "?"
}

type DeviceInfo struct {
	Name              string
	Probed            bool
	NumOutputChannels int
	NumInputChannels  int
	NumDuplexChannels int
	IsDefaultOutput   bool
	IsDefaultInput    bool

	PreferredSampleRate uint
	SampleRates         []int
}

type StreamOptions struct {
	channels     C.uint
	fs           C.uint
	bufferFrames C.uint
	time 		 C.uint
}

type StreamParameters struct {
	device_id     C.uint
	num_channels  C.uint
	first_channel C.uint
}

type sample_t C.int16_t
// TODO: Hard code for now
const sizeof_int16_t = 2

// Go doesn't need to redefine InputData - use C.InputData from the cgo block

// writeWavFile writes audio data to a WAV file
func writeWavFile(filename string, data *C.InputData, sampleRate, channels, bitsPerSample uint32) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Calculate sizes
	dataSize := uint32(data.frameCounter) * channels * (bitsPerSample / 8)

	// Write RIFF header
	file.Write([]byte("RIFF"))
	binary.Write(file, binary.LittleEndian, uint32(36+dataSize)) // ChunkSize
	file.Write([]byte("WAVE"))

	// Write fmt subchunk
	file.Write([]byte("fmt "))
	binary.Write(file, binary.LittleEndian, uint32(16))          // Subchunk1Size (PCM)
	binary.Write(file, binary.LittleEndian, uint16(1))           // AudioFormat (PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))    // NumChannels
	binary.Write(file, binary.LittleEndian, uint32(sampleRate))  // SampleRate
	binary.Write(file, binary.LittleEndian, uint32(sampleRate*channels*(bitsPerSample/8))) // ByteRate
	binary.Write(file, binary.LittleEndian, uint16(channels*(bitsPerSample/8)))           // BlockAlign
	binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)) // BitsPerSample

	// Write data subchunk
	file.Write([]byte("data"))
	binary.Write(file, binary.LittleEndian, uint32(dataSize)) // Subchunk2Size

	// Write audio data
	// Convert C array to Go slice
	totalSamples := int(data.frameCounter) * int(channels)
	samples := unsafe.Slice(data.buffer, totalSamples)

	// Write samples
	for _, sample := range samples {
		binary.Write(file, binary.LittleEndian, sample)
	}

	return nil
}

func Record() {
	options := StreamOptions {
		channels: C.uint(1),
		fs: C.uint(48000),
		bufferFrames: C.uint(512),
		time: C.uint(10),
	}

	audio := C.rtaudio_create(C.RTAUDIO_API_UNSPECIFIED)
	defer C.rtaudio_destroy(audio)
	deviceCount := C.rtaudio_device_count(audio)

	fmt.Printf("Options: %#v\n", options)
	fmt.Printf("Device count: %d\n\n", int(deviceCount))

	// List all available devices
	fmt.Println("Available audio devices:")
	for i := 0; i < int(deviceCount); i++ {
		deviceId := C.rtaudio_get_device_id(audio, C.int(i))
		deviceInfo := C.rtaudio_get_device_info(audio, deviceId)

		name := C.GoString(&deviceInfo.name[0])
		fmt.Printf("  [%d] %s\n", deviceId, name)
		fmt.Printf("      Input channels: %d, Output channels: %d\n",
			int(deviceInfo.input_channels), int(deviceInfo.output_channels))
		if deviceInfo.is_default_input != 0 {
			fmt.Printf("      (DEFAULT INPUT)\n")
		}
		if deviceInfo.is_default_output != 0 {
			fmt.Printf("      (DEFAULT OUTPUT)\n")
		}
	}
	fmt.Println()

	// Use C struct types directly
	var inputParams C.rtaudio_stream_parameters_t
	defaultInputDevice := C.rtaudio_get_default_input_device(audio)
	inputParams.device_id = defaultInputDevice
	inputParams.num_channels = options.channels
	inputParams.first_channel = 0

	fmt.Printf("Using input device ID: %d\n", int(defaultInputDevice))

	// Create C.InputData
	var data C.InputData
	data.totalFrames = C.ulong(options.fs * options.time)
	data.frameCounter = 0
	data.channels = options.channels
	data.bufferBytes = C.ulong(options.bufferFrames * options.channels * sizeof_int16_t)

	totalBytes := C.size_t(data.totalFrames * C.ulong(options.channels * sizeof_int16_t))
	data.buffer = (*C.sample_t)(C.malloc(totalBytes))
	defer C.free(unsafe.Pointer(data.buffer))

    // Open the stream
	err := C.rtaudio_open_stream(
        audio,
        nil,                        // no output
        &inputParams,               // input parameters
        C.RTAUDIO_FORMAT_SINT16,    // format
        options.fs,                 // sample rate
        &options.bufferFrames,      // buffer frames
        (*[0]byte)(C.input_callback), // callback function
        unsafe.Pointer(&data),      // user data
        nil,                        // stream options
        nil,                        // error callback
    )
	defer C.rtaudio_close_stream(audio)

    if err != C.RTAUDIO_ERROR_NONE {
        fmt.Printf("Error opening stream: %s\n", C.GoString(C.rtaudio_error(audio)))
		return
    }

	// Start the stream
	err = C.rtaudio_start_stream(audio)
	if err != C.RTAUDIO_ERROR_NONE {
		fmt.Printf("Error starting stream: %s\n", C.GoString(C.rtaudio_error(audio)))
		return
	}

    fmt.Printf("\nRecording for %d seconds ... (buffer frames = %d)\n", int(options.time), int(options.bufferFrames))

	// Wait for recording to complete, showing progress
	for i := 0; i < int(options.time); i++ {
		time.Sleep(1 * time.Second)
		fmt.Printf("  %d seconds elapsed, frames recorded: %d\n", i+1, int(data.frameCounter))
	}

	fmt.Printf("\nRecording complete. Recorded %d frames out of %d expected.\n",
		int(data.frameCounter), int(data.totalFrames))

	// Write WAV file
	wavFilename := "record.wav"
	fmt.Printf("Writing WAV file: %s\n", wavFilename)
	if err := writeWavFile(wavFilename, &data, uint32(options.fs), uint32(options.channels), 16); err != nil {
		fmt.Printf("Error writing WAV file: %v\n", err)
		return
	}
	fmt.Printf("Successfully wrote %s\n", wavFilename)
}

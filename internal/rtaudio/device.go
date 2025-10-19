package rtaudio

//go:generate go run build.go

/*
#cgo CXXFLAGS: -std=c++11 -g
#cgo CFLAGS: -g
#cgo windows CXXFLAGS: -D__WINDOWS_WASAPI__
#cgo windows CFLAGS: -D__WINDOWS_WASAPI__
#cgo windows LDFLAGS: ${SRCDIR}/rtaudio_go.o -lstdc++ -lm -lole32 -lwinmm -lksuser -lmfplat -lmfuuid -lwmcodecdspuuid -static -g
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

extern int goCallback(void *out, void *in, unsigned int nFrames,
	double stream_time, rtaudio_stream_status_t status, void *userdata);

static inline void cgoRtAudioOpenStream(rtaudio_t audio,
	rtaudio_stream_parameters_t *output_params,
	rtaudio_stream_parameters_t *input_params,
	rtaudio_format_t format,
	unsigned int sample_rate,
	unsigned int *buffer_frames,
	int cb_id,
	rtaudio_stream_options_t *options) {
		rtaudio_open_stream(audio, output_params, input_params,
			format, sample_rate, buffer_frames,
			goCallback, (void *)(uintptr_t)cb_id, options, NULL);
}
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"unsafe"
)

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
	RTAUDIO_ERROR_NONE              RtaudioError = C.RTAUDIO_ERROR_NONE              /*!< No error. */
	RTAUDIO_ERROR_WARNING                        = C.RTAUDIO_ERROR_WARNING           /*!< A non-critical error. */
	RTAUDIO_ERROR_UNKNOWN                        = C.RTAUDIO_ERROR_UNKNOWN           /*!< An unspecified error type. */
	RTAUDIO_ERROR_NO_DEVICES_FOUND               = C.RTAUDIO_ERROR_NO_DEVICES_FOUND  /*!< No devices found on system. */
	RTAUDIO_ERROR_INVALID_DEVICE                 = C.RTAUDIO_ERROR_INVALID_DEVICE    /*!< An invalid device ID was specified. */
	RTAUDIO_ERROR_DEVICE_DISCONNECT              = C.RTAUDIO_ERROR_DEVICE_DISCONNECT /*!< A device in use was disconnected. */
	RTAUDIO_ERROR_MEMORY_ERROR                   = C.RTAUDIO_ERROR_MEMORY_ERROR      /*!< An error occurred during memory allocation. */
	RTAUDIO_ERROR_INVALID_PARAMETER              = C.RTAUDIO_ERROR_INVALID_PARAMETER /*!< An invalid parameter was specified to a function. */
	RTAUDIO_ERROR_INVALID_USE                    = C.RTAUDIO_ERROR_INVALID_USE       /*!< The function was called incorrectly. */
	RTAUDIO_ERROR_DRIVER_ERROR                   = C.RTAUDIO_ERROR_DRIVER_ERROR      /*!< A system driver error occurred. */
	RTAUDIO_ERROR_SYSTEM_ERROR                   = C.RTAUDIO_ERROR_SYSTEM_ERROR      /*!< A system error occurred. */
	RTAUDIO_ERROR_THREAD_ERROR                   = C.RTAUDIO_ERROR_THREAD_ERROR      /*!< A thread error occurred. */
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

// StreamStatus defines over- or underflow flags in the audio callback.
type StreamStatus C.rtaudio_stream_status_t

const (
	// StatusInputOverflow indicates that data was discarded because of an
	// overflow condition at the driver.
	StatusInputOverflow StreamStatus = C.RTAUDIO_STATUS_INPUT_OVERFLOW
	// StatusOutputUnderflow indicates that the output buffer ran low, likely
	// producing a break in the output sound.
	StatusOutputUnderflow StreamStatus = C.RTAUDIO_STATUS_OUTPUT_UNDERFLOW
)

// Version returns current RtAudio library version string.
func Version() string {
	return C.GoString(C.rtaudio_version())
}

// CompiledAPI determines the available compiled audio APIs.
func CompiledAPI() (apis []API) {
	capis := (*[1 << 27]C.rtaudio_api_t)(unsafe.Pointer(C.rtaudio_compiled_api()))
	for i := 0; ; i++ {
		api := capis[i]
		if api == C.RTAUDIO_API_UNSPECIFIED {
			break
		}
		apis = append(apis, API(api))
	}
	return apis
}

type DeviceInfo struct {
	Name              string
	NumOutputChannels int
	NumInputChannels  int
	NumDuplexChannels int
	IsDefaultOutput   bool
	IsDefaultInput    bool

	PreferredSampleRate uint
	SampleRates         []int
}

type StreamFlags C.rtaudio_stream_flags_t

const (
	// FlagsNoninterleaved is set to use non-interleaved buffers (default = interleaved).
	FlagsNoninterleaved = C.RTAUDIO_FLAGS_NONINTERLEAVED
	// FlagsMinimizeLatency when set attempts to configure stream parameters for lowest possible latency.
	FlagsMinimizeLatency = C.RTAUDIO_FLAGS_MINIMIZE_LATENCY
	// FlagsHogDevice when set attempts to grab device for exclusive use.
	FlagsHogDevice = C.RTAUDIO_FLAGS_HOG_DEVICE
	// FlagsScheduleRealtime is set in attempt to select realtime scheduling (round-robin) for the callback thread.
	FlagsScheduleRealtime = C.RTAUDIO_FLAGS_SCHEDULE_REALTIME
	// FlagsAlsaUseDefault is set to use the "default" PCM device (ALSA only).
	FlagsAlsaUseDefault = C.RTAUDIO_FLAGS_ALSA_USE_DEFAULT
)

// StreamOptions is the structure for specifying stream options.
type StreamOptions struct {
	Flags      StreamFlags
	NumBuffers uint
	Priotity   int
	Name       string
}

type StreamParameters struct {
	device_id     C.uint
	num_channels  C.uint
	first_channel C.uint
}

// StreamParams is the structure for specifying input or output stream parameters.
type StreamParams struct {
	DeviceID     uint
	NumChannels  uint
	FirstChannel uint
}

type sample_t C.int16_t

// TODO: Hard code for now
const sizeof_int16_t = 2

// RtAudio is a "controller" used to select an available audio i/o interface.
type RtAudio interface {
	Destroy()
	CurrentAPI() API
	Devices() ([]DeviceInfo, error)
	DefaultOutputDeviceId() int
	DefaultInputDeviceId() int
	DefaultOutputDevice() DeviceInfo
	DefaultInputDevice() DeviceInfo

	Open(out, in *StreamParams, format Format, sampleRate uint, frames uint, cb Callback, opts *StreamOptions) error
	Close()
	Start() error
	Stop() error
	Abort() error

	IsOpen() bool
	IsRunning() bool

	Latency() (int, error)
	SampleRate() (uint, error)
	Time() (time.Duration, error)
	SetTime(time.Duration) error

	ShowWarnings(bool)
}

// Go doesn't need to redefine InputData - use C.InputData from the cgo block

// Callback is a client-defined function that will be invoked when input data
// is available and/or output data is needed.
type Callback func(out Buffer, in Buffer, dur time.Duration, status StreamStatus) int
type rtaudio struct {
	audio          C.rtaudio_t
	cb             Callback
	inputChannels  int
	outputChannels int
	format         Format
}

// Create a new RtAudio instance using the given API.
func Create(api API) (RtAudio, error) {
	audio := C.rtaudio_create(C.rtaudio_api_t(api))
	if C.rtaudio_error(audio) != nil {
		return nil, errors.New(C.GoString(C.rtaudio_error(audio)))
	}
	return &rtaudio{audio: audio}, nil
}

func (audio *rtaudio) Destroy() {
	C.rtaudio_destroy(audio.audio)
}

func (audio *rtaudio) CurrentAPI() API {
	return API(C.rtaudio_current_api(audio.audio))
}

func (audio *rtaudio) DefaultInputDeviceId() int {
	return int(C.rtaudio_get_default_input_device(audio.audio))
}

func (audio *rtaudio) DefaultOutputDeviceId() int {
	return int(C.rtaudio_get_default_output_device(audio.audio))
}

// TODO: This may be broken
func (audio *rtaudio) DefaultInputDevice() DeviceInfo {
	devices, err := audio.Devices()
	if err != nil {
		log.Fatal("Failed to initialise devices")
	}
	// Find the default input device by checking the IsDefaultInput flag
	var defaultIn DeviceInfo
	found := false
	for i := range devices {

		fmt.Printf("%v\n", devices[i])
		if devices[i].IsDefaultInput {
			defaultIn = devices[i]
			found = true
			break
		}
	}
	if !found {
		log.Fatal("No default input device found")
	}
	return defaultIn
}

func (audio *rtaudio) DefaultOutputDevice() DeviceInfo {
	devices, err := audio.Devices()
	if err != nil {
		log.Fatal("No default input device found")
	}
	// Find the default input device by checking the IsDefaultOutput flag
	var defaultOut DeviceInfo
	found := false
	for i := range devices {
		if devices[i].IsDefaultOutput {
			defaultOut = devices[i]
			found = true
			break
		}
	}
	if !found {
		log.Fatal("No default input device found")
	}
	return defaultOut
}

func (audio *rtaudio) Devices() ([]DeviceInfo, error) {
	n := C.rtaudio_device_count(audio.audio)
	devices := []DeviceInfo{}
	for i := C.int(0); i < n; i++ {

		deviceId := C.rtaudio_get_device_id(audio.audio, C.int(i))
		cinfo := C.rtaudio_get_device_info(audio.audio, deviceId)
		if C.rtaudio_error(audio.audio) != nil {
			return nil, errors.New(C.GoString(C.rtaudio_error(audio.audio)))
		}
		sr := []int{}
		for _, r := range cinfo.sample_rates {
			if r == 0 {
				break
			}
			sr = append(sr, int(r))
		}
		devices = append(devices, DeviceInfo{
			Name:                C.GoString(&cinfo.name[0]),
			NumInputChannels:    int(cinfo.input_channels),
			NumOutputChannels:   int(cinfo.output_channels),
			NumDuplexChannels:   int(cinfo.duplex_channels),
			IsDefaultOutput:     cinfo.is_default_output != 0,
			IsDefaultInput:      cinfo.is_default_input != 0,
			PreferredSampleRate: uint(cinfo.preferred_sample_rate),
			SampleRates:         sr,
		})
		// TODO: formats
	}
	return devices, nil
}

type Format int

const (
	// FormatInt8 uses 8-bit signed integer.
	FormatInt8 Format = C.RTAUDIO_FORMAT_SINT8
	// FormatInt16 uses 16-bit signed integer.
	FormatInt16 = C.RTAUDIO_FORMAT_SINT16
	// FormatInt24 uses 24-bit signed integer.
	FormatInt24 = C.RTAUDIO_FORMAT_SINT24
	// FormatInt32 uses 32-bit signed integer.
	FormatInt32 = C.RTAUDIO_FORMAT_SINT32
	// FormatFloat32 uses 32-bit floating point values normalized between (-1..1).
	FormatFloat32 = C.RTAUDIO_FORMAT_FLOAT32
	// FormatFloat64 uses 64-bit floating point values normalized between (-1..1).
	FormatFloat64 = C.RTAUDIO_FORMAT_FLOAT64
)

// Buffer is a common interface for audio buffers of various data format types.
type Buffer interface {
	Len() int
	Int8() []int8
	Int16() []int16
	Int24() []Int24
	Int32() []int32
	Float32() []float32
	Float64() []float64
}

// Int24 is a helper type to convert int32 values to int24 and back.
type Int24 [3]byte

// Set Int24 value using the least significant bytes of the given number n.
func (i *Int24) Set(n int32) {
	(*i)[0], (*i)[1], (*i)[2] = byte(n&0xff), byte((n&0xff00)>>8), byte((n&0xff0000)>>16)
}

// Get Int24 value as int32.
func (i Int24) Get() int32 {
	n := int32(i[0]) | int32(i[1])<<8 | int32(i[2])<<16
	if n&0x800000 != 0 {
		n |= ^0xffffff
	}
	return n
}

type buffer struct {
	format      Format
	length      int
	numChannels int
	ptr         unsafe.Pointer
}

func (b *buffer) Len() int {
	if b.ptr == nil {
		return 0
	}
	return b.length
}

func (b *buffer) Int8() []int8 {
	if b.format != FormatInt8 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 30]int8)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

func (b *buffer) Int16() []int16 {
	if b.format != FormatInt16 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 29]int16)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

func (b *buffer) Int24() []Int24 {
	if b.format != FormatInt24 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 28]Int24)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

func (b *buffer) Int32() []int32 {
	if b.format != FormatInt32 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 27]int32)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

func (b *buffer) Float32() []float32 {
	if b.format != FormatFloat32 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 27]float32)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

func (b *buffer) Float64() []float64 {
	if b.format != FormatFloat64 {
		return nil
	}
	if b.ptr == nil {
		return nil
	}
	return (*[1 << 23]float64)(b.ptr)[: b.length*b.numChannels : b.length*b.numChannels]
}

var (
	mu     sync.Mutex
	audios = map[int]*rtaudio{}
)

func registerAudio(a *rtaudio) int {
	mu.Lock()
	defer mu.Unlock()
	for i := 0; ; i++ {
		if _, ok := audios[i]; !ok {
			audios[i] = a
			return i
		}
	}
}

func unregisterAudio(a *rtaudio) {
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < len(audios); i++ {
		if audios[i] == a {
			delete(audios, i)
			return
		}
	}
}

func findAudio(k int) *rtaudio {
	mu.Lock()
	defer mu.Unlock()
	return audios[k]
}

//export goCallback
func goCallback(out, in unsafe.Pointer, frames C.uint, sec C.double,
	status C.rtaudio_stream_status_t, userdata unsafe.Pointer) C.int {

	k := int(uintptr(userdata))
	audio := findAudio(k)
	dur := time.Duration(time.Microsecond * time.Duration(sec*1000000.0))
	inbuf := &buffer{audio.format, int(frames), audio.inputChannels, in}
	outbuf := &buffer{audio.format, int(frames), audio.outputChannels, out}
	return C.int(audio.cb(outbuf, inbuf, dur, StreamStatus(status)))
}

func (audio *rtaudio) Open(out, in *StreamParams, format Format, sampleRate uint,
	frames uint, cb Callback, opts *StreamOptions) error {
	var (
		cInPtr   *C.rtaudio_stream_parameters_t
		cOutPtr  *C.rtaudio_stream_parameters_t
		cOptsPtr *C.rtaudio_stream_options_t
		cIn      C.rtaudio_stream_parameters_t
		cOut     C.rtaudio_stream_parameters_t
		cOpts    C.rtaudio_stream_options_t
	)

	audio.inputChannels = 0
	audio.outputChannels = 0
	if out != nil {
		audio.outputChannels = int(out.NumChannels)
		cOut.device_id = C.uint(out.DeviceID)
		cOut.num_channels = C.uint(out.NumChannels)
		cOut.first_channel = C.uint(out.FirstChannel)
		cOutPtr = &cOut
	}
	if in != nil {
		audio.inputChannels = int(in.NumChannels)
		cIn.device_id = C.uint(in.DeviceID)
		cIn.num_channels = C.uint(in.NumChannels)
		cIn.first_channel = C.uint(in.FirstChannel)
		cInPtr = &cIn
	}
	if opts != nil {
		cOpts.flags = C.rtaudio_stream_flags_t(opts.Flags)
		cOpts.num_buffers = C.uint(opts.NumBuffers)
		cOpts.priority = C.int(opts.Priotity)
		cOptsPtr = &cOpts
	}
	framesCount := C.uint(frames)
	audio.format = format
	audio.cb = cb

	k := registerAudio(audio)
	C.cgoRtAudioOpenStream(audio.audio, cOutPtr, cInPtr,
		C.rtaudio_format_t(format), C.uint(sampleRate), &framesCount, C.int(k), cOptsPtr)
	if C.rtaudio_error(audio.audio) != nil {
		return errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return nil
}

func (audio *rtaudio) Close() {
	unregisterAudio(audio)
	C.rtaudio_close_stream(audio.audio)
}

func (audio *rtaudio) Start() error {
	C.rtaudio_start_stream(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return nil
}

func (audio *rtaudio) Stop() error {
	C.rtaudio_stop_stream(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return nil
}

func (audio *rtaudio) Abort() error {
	C.rtaudio_abort_stream(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return nil
}

func (audio *rtaudio) IsOpen() bool {
	return C.rtaudio_is_stream_open(audio.audio) != 0
}

func (audio *rtaudio) IsRunning() bool {
	return C.rtaudio_is_stream_running(audio.audio) != 0
}

func (audio *rtaudio) Latency() (int, error) {
	latency := C.rtaudio_get_stream_latency(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return 0, errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return int(latency), nil
}

func (audio *rtaudio) SampleRate() (uint, error) {
	sampleRate := C.rtaudio_get_stream_sample_rate(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return 0, errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return uint(sampleRate), nil
}

func (audio *rtaudio) Time() (time.Duration, error) {
	sec := C.rtaudio_get_stream_time(audio.audio)
	if C.rtaudio_error(audio.audio) != nil {
		return 0, errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return time.Duration(time.Microsecond * time.Duration(sec*1000000.0)), nil
}

func (audio *rtaudio) SetTime(t time.Duration) error {
	sec := float64(t) * 1000000.0 / float64(time.Microsecond)
	C.rtaudio_set_stream_time(audio.audio, C.double(sec))
	if C.rtaudio_error(audio.audio) != nil {
		return errors.New(C.GoString(C.rtaudio_error(audio.audio)))
	}
	return nil
}

func (audio *rtaudio) ShowWarnings(show bool) {
	if show {
		C.rtaudio_show_warnings(audio.audio, 1)
	} else {
		C.rtaudio_show_warnings(audio.audio, 0)
	}
}

// writeWavFile writes audio data to a WAV file
func writeWavFile(filename string, data *RecordingData, sampleRate uint32, bitsPerSample uint32) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	channels := uint32(data.channels)

	// Calculate sizes based on actual frames recorded
	dataSize := uint32(data.frameCounter) * channels * (bitsPerSample / 8)

	// Write RIFF header
	file.Write([]byte("RIFF"))
	binary.Write(file, binary.LittleEndian, uint32(36+dataSize)) // ChunkSize
	file.Write([]byte("WAVE"))

	// Write fmt subchunk
	file.Write([]byte("fmt "))
	binary.Write(file, binary.LittleEndian, uint32(16))                                    // Subchunk1Size (PCM)
	binary.Write(file, binary.LittleEndian, uint16(1))                                     // AudioFormat (PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))                              // NumChannels
	binary.Write(file, binary.LittleEndian, uint32(sampleRate))                            // SampleRate
	binary.Write(file, binary.LittleEndian, uint32(sampleRate*channels*(bitsPerSample/8))) // ByteRate
	binary.Write(file, binary.LittleEndian, uint16(channels*(bitsPerSample/8)))            // BlockAlign
	binary.Write(file, binary.LittleEndian, uint16(bitsPerSample))                         // BitsPerSample

	// Write data subchunk
	file.Write([]byte("data"))
	binary.Write(file, binary.LittleEndian, uint32(dataSize)) // Subchunk2Size

	// Write audio data from Go slice
	totalSamples := data.frameCounter * data.channels
	for i := 0; i < totalSamples; i++ {
		if err := binary.Write(file, binary.LittleEndian, data.buffer[i]); err != nil {
			return fmt.Errorf("failed to write sample: %w", err)
		}
	}

	return nil
}


//TODO: Below is AI idea of what the API could look like for actual streaming.
//    Currently untested but here as a rough outline

// RecordingData holds the state for audio recording
type RecordingData struct {
	buffer       []int16
	totalFrames  int
	frameCounter int
	channels     int
}

// AudioChunk represents a chunk of audio data from the stream
type AudioChunk struct {
	Data      []int16       // Audio samples (interleaved if multi-channel)
	Frames    int           // Number of frames in this chunk
	Channels  int           // Number of channels
	Timestamp time.Duration // Stream time when this chunk was captured
	Status    StreamStatus  // Any status flags (overflow, etc.)
}

// AudioStream represents an active audio input stream
type AudioStream struct {
	audio      RtAudio
	Data       <-chan AudioChunk // Read audio chunks from this channel
	Errors     <-chan error      // Listen for errors
	done       chan struct{}     // Internal: signal to stop
	dataWrite  chan AudioChunk   // Internal: write side of Data channel
	errWrite   chan error        // Internal: write side of Errors channel
	sampleRate uint
	channels   int
}

// Stop gracefully stops the audio stream
func (s *AudioStream) Stop() error {
	close(s.done)
	if s.audio.IsRunning() {
		if err := s.audio.Stop(); err != nil {
			return err
		}
	}
	s.audio.Close()
	close(s.dataWrite)
	close(s.errWrite)
	return nil
}

// SampleRate returns the sample rate of the stream
func (s *AudioStream) SampleRate() uint {
	return s.sampleRate
}

// Channels returns the number of channels in the stream
func (s *AudioStream) Channels() int {
	return s.channels
}

// StartStreaming starts an indefinite audio input stream on the default input device
// The stream will continue until Stop() is called
// Audio data is sent through the returned AudioStream.Data channel
func StartStreaming(bufferFrames uint) (*AudioStream, error) {
	audio, err := Create(APIUnspecified)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio interface: %w", err)
	}

	defaultIn := audio.DefaultInputDevice()
	channels := defaultIn.NumInputChannels
	sampleRate := defaultIn.PreferredSampleRate

	// Create channels for communication
	dataWrite := make(chan AudioChunk, 10) // Buffer up to 10 chunks
	errWrite := make(chan error, 5)
	done := make(chan struct{})

	stream := &AudioStream{
		audio:      audio,
		Data:       dataWrite,
		Errors:     errWrite,
		done:       done,
		dataWrite:  dataWrite,
		errWrite:   errWrite,
		sampleRate: sampleRate,
		channels:   channels,
	}

	params := StreamParams{
		DeviceID:     uint(audio.DefaultInputDeviceId()),
		NumChannels:  uint(channels),
		FirstChannel: 0,
	}

	options := StreamOptions{
		Flags: FlagsScheduleRealtime | FlagsMinimizeLatency,
	}

	// Callback that sends data to the channel
	cb := func(out, in Buffer, dur time.Duration, status StreamStatus) int {
		// Check if we should stop
		select {
		case <-done:
			return 2 // Stop the stream
		default:
		}

		inputData := in.Int16()
		if inputData == nil {
			return 0
		}

		nFrames := in.Len()

		// TODO: This seems like an allocation we don't want and should be able to reuse some preallocated memory?
		// Make a copy of the data since the buffer is reused
		dataCopy := make([]int16, len(inputData))
		copy(dataCopy, inputData)

		chunk := AudioChunk{
			Data:      dataCopy,
			Frames:    nFrames,
			Channels:  channels,
			Timestamp: dur,
			Status:    status,
		}

		// Send the chunk, but don't block if the channel is full
		select {
		case dataWrite <- chunk:
		default:
			// Channel full - data is being dropped
			// Could log this or send to error channel
		}

		return 0
	}

	err = audio.Open(nil, &params, FormatInt16, sampleRate, bufferFrames, cb, &options)
	if err != nil {
		audio.Destroy()
		return nil, fmt.Errorf("failed to open audio stream: %w", err)
	}

	err = audio.Start()
	if err != nil {
		audio.Close()
		audio.Destroy()
		return nil, fmt.Errorf("failed to start audio stream: %w", err)
	}

	return stream, nil
}

// TODO: Rough idea of how it would be used in the network code.
// // Start the stream
// stream, err := audio.StartStreaming(512)
//
//	if err != nil {
//	    return err
//	}
//
// defer stream.Stop()
//
// // Consume audio in your network handler
//
//	go func() {
//	    for chunk := range stream.Data {
//	        // Encode and send over network
//	        encoded := encodeToOpus(chunk.Data)
//	        websocket.Send(encoded)
//	    }
//	}()
//
// StreamExample demonstrates how to use the streaming API
// This shows how your networking code could consume audio data
func StreamExample() {
	// Start streaming with 512 frame buffer
	stream, err := StartStreaming(512)
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Stop()

	fmt.Printf("Streaming audio: %d Hz, %d channels\n", stream.SampleRate(), stream.Channels())
	fmt.Println("Press Ctrl+C to stop...")

	// Process audio chunks as they arrive
	chunkCount := 0
	for {
		select {
		case chunk, ok := <-stream.Data:
			if !ok {
				fmt.Println("Stream closed")
				return
			}
			chunkCount++

			// Here's where you would send the audio to your network layer
			// For example:
			//   - Encode to opus/mp3
			//   - Send over websocket/UDP
			//   - Process with ML model
			//   - etc.

			fmt.Printf("\rReceived chunk #%d: %d frames, %d samples",
				chunkCount, chunk.Frames, len(chunk.Data))

			// Example: Check for overflow
			if chunk.Status&StatusInputOverflow != 0 {
				fmt.Println("\nWARNING: Input overflow detected!")
			}

		case err := <-stream.Errors:
			fmt.Printf("\nError: %v\n", err)
			return
		}
	}
}

// OutputData holds the state for audio playback
type OutputData struct {
	file         *os.File
	channels     int
	frameCounter int
	totalFrames  int
}

// readWavHeader reads a WAV file header and returns the audio format information
func readWavHeader(file *os.File) (channels, sampleRate, bitsPerSample, dataSize int, err error) {
	var riffHeader [12]byte
	if _, err = file.Read(riffHeader[:]); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to read RIFF header: %w", err)
	}

	// Check RIFF header
	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return 0, 0, 0, 0, fmt.Errorf("not a valid WAV file")
	}

	// Read fmt subchunk
	var fmtHeader [8]byte
	if _, err = file.Read(fmtHeader[:]); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to read fmt header: %w", err)
	}

	if string(fmtHeader[0:4]) != "fmt " {
		return 0, 0, 0, 0, fmt.Errorf("fmt chunk not found")
	}

	fmtSize := binary.LittleEndian.Uint32(fmtHeader[4:8])
	fmtData := make([]byte, fmtSize)
	if _, err = file.Read(fmtData); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to read fmt data: %w", err)
	}

	audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
	if audioFormat != 1 {
		return 0, 0, 0, 0, fmt.Errorf("only PCM format is supported")
	}

	channels = int(binary.LittleEndian.Uint16(fmtData[2:4]))
	sampleRate = int(binary.LittleEndian.Uint32(fmtData[4:8]))
	bitsPerSample = int(binary.LittleEndian.Uint16(fmtData[14:16]))

	// Find data chunk
	var chunkHeader [8]byte
	for {
		if _, err = file.Read(chunkHeader[:]); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("failed to find data chunk: %w", err)
		}

		chunkSize := int(binary.LittleEndian.Uint32(chunkHeader[4:8]))

		if string(chunkHeader[0:4]) == "data" {
			dataSize = chunkSize
			break
		}

		// Skip this chunk
		if _, err = file.Seek(int64(chunkSize), 1); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("failed to skip chunk: %w", err)
		}
	}

	return channels, sampleRate, bitsPerSample, dataSize, nil
}


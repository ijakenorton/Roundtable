package device

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
	"github.com/Honorable-Knights-of-the-Roundtable/rtaudiowrapper"
	"github.com/google/uuid"
)

// RtAudioInputDevice is an AudioInputDevice that captures audio from a microphone using RtAudio.
// It implements the AudioSourceDevice interface.
type RtAudioInputDevice struct {
	logger *slog.Logger
	uuid   uuid.UUID

	audio        rtaudiowrapper.RtAudio
	sampleRate   uint
	numChannels  int
	dataChannel  chan frame.PCMFrame
	errorChannel chan error
	framesLost   atomic.Uint64
	DeviceID     int

	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	shutdownOnce  sync.Once
}

// NewRtAudioInputDevice creates a new RtAudioInputDevice using the default input device.
// bufferFrames determines the size of audio chunks (typically 512 or 1024).
func (api *RtAudioApi) InitInputDeviceFromID(frameDuration time.Duration, id int) (*RtAudioInputDevice, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"rtaudio input device uuid", uuid,
	)

	audio, err := rtaudiowrapper.Create(rtaudiowrapper.APIUnspecified)
	if err != nil {
		logger.Error("failed to create rtaudio interface", "err", err)
		return nil, fmt.Errorf("failed to create audio interface: %w", err)
	}

	// Find Picked Device
	devices, err := audio.Devices()
	if err != nil {
		logger.Error("failed to get devices", "err", err)
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var currentDevice *rtaudiowrapper.DeviceInfo
	for _, d := range devices {
		if d.ID == id {
			currentDevice = &d
			break
		}
	}

	if currentDevice == nil {
		return nil, fmt.Errorf("device with ID %d not found", id)
	}

	name := currentDevice.Name
	numChannels := currentDevice.NumInputChannels
	sampleRate := currentDevice.PreferredSampleRate

	bufferFrames := uint(int(sampleRate) * int(frameDuration) / int(time.Second))
	logger.Debug(
		"initialized rtaudio input device",
		"device", name,
		"sampleRate", sampleRate,
		"channels", numChannels,
		"bufferFrames", bufferFrames,
	)

	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	dataChannel := make(chan frame.PCMFrame)
	errorChannel := make(chan error, 5)

	device := &RtAudioInputDevice{
		logger:        logger,
		uuid:          uuid,
		DeviceID:      id,
		audio:         audio,
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		dataChannel:   dataChannel,
		errorChannel:  errorChannel,
		framesLost:    atomic.Uint64{},
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
	}

	// Set up stream parameters
	params := rtaudiowrapper.StreamParams{
		DeviceID:     uint(id),
		NumChannels:  uint(numChannels),
		FirstChannel: 0,
	}

	options := rtaudiowrapper.StreamOptions{
		Flags: rtaudiowrapper.FlagsScheduleRealtime | rtaudiowrapper.FlagsMinimizeLatency,
	}

	cb := func(out, in rtaudiowrapper.Buffer, dur time.Duration, status rtaudiowrapper.StreamStatus) int {
		// Check if we should stop
		select {
		case <-ctx.Done():
			return 2 // Stop the stream
		default:
		}

		inputData := in.Float32()
		if inputData == nil {
			return 0
		}
		// Convert float32 slice to PCMFrame (already in correct format)
		pcmFrame := make(frame.PCMFrame, len(inputData))
		copy(pcmFrame, inputData)
		dataChannel <- pcmFrame

		// Check for input overflow
		if status&rtaudiowrapper.StatusInputOverflow != 0 {
			logger.Warn("input overflow detected")
		}

		return 0
	}

	err = audio.Open(nil, &params, rtaudiowrapper.FormatFloat32, sampleRate, bufferFrames, cb, &options)
	if err != nil {
		// TODO Unsure if it is ok to have the shared pointer to audio
		audio.Destroy()
		logger.Error("failed to open audio stream", "err", err)
		return nil, fmt.Errorf("failed to open audio stream: %w", err)
	}

	err = audio.Start()
	if err != nil {
		audio.Close()
		audio.Destroy()
		logger.Error("failed to start audio stream", "err", err)
		return nil, fmt.Errorf("failed to start audio stream: %w", err)
	}

	logger.Info("rtaudio input device started successfully")

	return device, nil
}

// NewRtAudioInputDevice creates a new RtAudioInputDevice using the default input device.
// bufferFrames determines the size of audio chunks (typically 512 or 1024).
func (api *RtAudioApi) InitDefaultInputDevice(frameDuration time.Duration) (*RtAudioInputDevice, error) {
	return api.InitInputDeviceFromID(frameDuration, api.audio.DefaultInputDeviceId())
}

// GetStream returns the channel that will receive PCM audio frames from the microphone.
func (d *RtAudioInputDevice) GetStream() <-chan frame.PCMFrame {
	return d.dataChannel
}

// Close stops the audio stream and cleans up resources.
func (d *RtAudioInputDevice) Close() {
	d.logger.Debug("shutdown called")
	d.shutdownOnce.Do(func() {
		if d.audio.IsRunning() {
			if err := d.audio.Stop(); err != nil {
				d.logger.Error("error stopping audio stream", "err", err)
			}
		}

		d.audio.Close()
		d.audio.Destroy()

		close(d.dataChannel)
		close(d.errorChannel)
		d.ctxCancelFunc()

		totalLost := d.framesLost.Load()
		if totalLost > 0 {
			d.logger.Warn("frames dropped during capture", "totalFrames", totalLost)
		}
		d.logger.Info("rtaudio input device closed")
	})
}

// GetDeviceProperties returns the audio properties (sample rate, channels) of this device.
func (d *RtAudioInputDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return audiodevice.DeviceProperties{
		SampleRate:  int(d.sampleRate),
		NumChannels: d.numChannels,
	}
}

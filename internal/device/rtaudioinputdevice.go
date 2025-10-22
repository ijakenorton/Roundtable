package device

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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
	done         chan struct{}

	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	shutdownOnce  sync.Once
}

// NewRtAudioInputDevice creates a new RtAudioInputDevice using the default input device.
// bufferFrames determines the size of audio chunks (typically 512 or 1024).
func NewRtAudioInputDevice(bufferFrames uint) (*RtAudioInputDevice, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"rtaudio input device uuid", uuid,
	)

	audio, err := rtaudiowrapper.Create(rtaudiowrapper.APIUnspecified)
	if err != nil {
		logger.Error("failed to create rtaudio interface", "err", err)
		return nil, fmt.Errorf("failed to create audio interface: %w", err)
	}

	defaultIn := audio.DefaultInputDevice()
	numChannels := defaultIn.NumInputChannels
	sampleRate := defaultIn.PreferredSampleRate

	logger.Debug(
		"initialized rtaudio input device",
		"device", defaultIn.Name,
		"sampleRate", sampleRate,
		"channels", numChannels,
		"bufferFrames", bufferFrames,
	)

	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	dataChannel := make(chan frame.PCMFrame, 10) // Buffer up to 10 chunks
	errorChannel := make(chan error, 5)
	done := make(chan struct{})

	device := &RtAudioInputDevice{
		logger:        logger,
		uuid:          uuid,
		audio:         audio,
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		dataChannel:   dataChannel,
		errorChannel:  errorChannel,
		done:          done,
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
	}

	// Set up stream parameters
	params := rtaudiowrapper.StreamParams{
		DeviceID:     uint(audio.DefaultInputDeviceId()),
		NumChannels:  uint(numChannels),
		FirstChannel: 0,
	}

	options := rtaudiowrapper.StreamOptions{
		Flags: rtaudiowrapper.FlagsScheduleRealtime | rtaudiowrapper.FlagsMinimizeLatency,
	}

	// Callback that sends data to the channel
	cb := func(out, in rtaudiowrapper.Buffer, dur time.Duration, status rtaudiowrapper.StreamStatus) int {
		// Check if we should stop
		select {
		case <-done:
			return 2 // Stop the stream
		default:
		}

		inputData := in.Float32()
		if inputData == nil {
			return 0
		}

		nFrames := in.Len()

		// Convert float32 slice to PCMFrame (already in correct format)
		pcmFrame := make(frame.PCMFrame, len(inputData))
		copy(pcmFrame, inputData)

		// Send the chunk, but don't block if the channel is full
		select {
		case dataChannel <- pcmFrame:
		default:
			// Channel full - data is being dropped
			logger.Warn("audio input buffer full, dropping frame",
				"frames", nFrames,
				"timestamp", dur,
			)
		}

		// Check for input overflow
		if status&rtaudiowrapper.StatusInputOverflow != 0 {
			logger.Warn("input overflow detected")
		}

		return 0
	}

	err = audio.Open(nil, &params, rtaudiowrapper.FormatFloat32, sampleRate, bufferFrames, cb, &options)
	if err != nil {
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

// GetStream returns the channel that will receive PCM audio frames from the microphone.
func (d *RtAudioInputDevice) GetStream() <-chan frame.PCMFrame {
	return d.dataChannel
}

// Close stops the audio stream and cleans up resources.
func (d *RtAudioInputDevice) Close() {
	d.logger.Debug("shutdown called")
	d.shutdownOnce.Do(func() {
		close(d.done)

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

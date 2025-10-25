package device

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
	"github.com/Honorable-Knights-of-the-Roundtable/rtaudiowrapper"
	"github.com/google/uuid"
)

// RtAudioOutputDevice is an AudioOutputDevice that plays audio to speakers using RtAudio.
// It implements the AudioSinkDevice interface.
type RtAudioOutputDevice struct {
	logger *slog.Logger
	uuid   uuid.UUID

	audio        rtaudiowrapper.RtAudio
	sampleRate   int
	numChannels  int
	dataChannel  <-chan frame.PCMFrame
	bufferFrames uint
	DeviceID     int

	// Internal buffer to handle streaming from channel to rtaudio callback
	frameQueue   chan frame.PCMFrame
	shutdownOnce sync.Once
	closeWg      sync.WaitGroup
}

// NewRtAudioOutputDevice creates a new RtAudioOutputDevice using the default output device.
// sampleRate and numChannels define the expected audio format.
// bufferFrames determines the size of audio chunks (typically 512 or 1024).
func (api *RtAudioApi) InitOutputDeviceFromID(frameDuration time.Duration, id int) (*RtAudioOutputDevice, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"rtaudio output device uuid", uuid,
	)

	audio, err := rtaudiowrapper.Create(rtaudiowrapper.APIUnspecified)
	if err != nil {
		logger.Error("failed to create rtaudio interface", "err", err)
		return nil, fmt.Errorf("failed to create audio interface: %w", err)
	}

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
	sampleRate := int(currentDevice.PreferredSampleRate)
	channels := currentDevice.NumOutputChannels
	bufferFrames := uint(int(sampleRate) * int(frameDuration) / int(time.Second))

	logger.Debug(
		"initialized rtaudio output device",
		"device", name,
		"sampleRate", sampleRate,
		"channels", channels,
		"bufferFrames", bufferFrames,
		"DeviceID", id,
	)

	device := &RtAudioOutputDevice{
		logger:       logger,
		uuid:         uuid,
		DeviceID:     id,
		audio:        audio,
		sampleRate:   sampleRate,
		numChannels:  channels,
		bufferFrames: bufferFrames,
		frameQueue:   make(chan frame.PCMFrame), // Buffer to smooth out playback
	}

	return device, nil
}

func (api *RtAudioApi) InitDefaultOutputDevice(frameDuration time.Duration) (*RtAudioOutputDevice, error) {
	return api.InitOutputDeviceFromID(frameDuration, api.audio.DefaultOutputDeviceId())
}

// SetStream sets the source channel for audio data and starts playback.
// This method starts the RtAudio stream and begins consuming PCM frames from the channel.
func (d *RtAudioOutputDevice) SetStream(sourceChannel <-chan frame.PCMFrame) {
	d.dataChannel = sourceChannel

	// Set up stream parameters for output
	params := rtaudiowrapper.StreamParams{
		DeviceID:     uint(d.DeviceID),
		NumChannels:  uint(d.numChannels),
		FirstChannel: 0,
	}

	// Output callback function
	cb := func(out rtaudiowrapper.Buffer, in rtaudiowrapper.Buffer, dur time.Duration, status rtaudiowrapper.StreamStatus) int {
		outputData := out.Float32()
		if outputData == nil {
			return 0
		}

		samplesGathered := 0
		pcmFrame, ok := <-d.frameQueue

		if !ok {
			// Channel closed, fill remaining with silence and stop
			for i := samplesGathered; i < len(outputData); i++ {
				outputData[i] = 0
			}
			return 2 // Stop stream
		}
		copy(outputData, pcmFrame)

		return 0
	}

	err := d.audio.Open(&params, nil, rtaudiowrapper.FormatFloat32, uint(d.sampleRate), d.bufferFrames, cb, nil)
	if err != nil {
		d.logger.Error("failed to open audio stream", "err", err)
		return
	}

	err = d.audio.Start()
	if err != nil {
		d.logger.Error("failed to start audio stream", "err", err)
		d.audio.Close()
		return
	}

	d.logger.Info("rtaudio output device started successfully")

	// Start goroutine to feed frames from source channel to internal queue
	d.closeWg.Add(1)
	go func() {
		defer d.closeWg.Done()
		defer close(d.frameQueue)

		for pcmFrame := range sourceChannel {
			select {
			case d.frameQueue <- pcmFrame:
			}
		}

		d.logger.Debug("source channel closed")
	}()
}

// Close stops the audio stream and cleans up resources.
func (d *RtAudioOutputDevice) Close() {
	d.logger.Debug("shutdown called")
	d.shutdownOnce.Do(func() {
		// Stop audio stream first
		if d.audio.IsRunning() {
			if err := d.audio.Stop(); err != nil {
				d.logger.Error("error stopping audio stream", "err", err)
			}
		}

		d.audio.Close()
		d.audio.Destroy()

		// Wait for the streaming goroutine to finish (with timeout)
		done := make(chan struct{})
		go func() {
			d.closeWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			d.logger.Info("rtaudio output device closed")
		case <-time.After(1 * time.Second):
			d.logger.Warn("timeout waiting for output device to close")
		}
	})
}

// GetDeviceProperties returns the audio properties (sample rate, channels) of this device.
func (d *RtAudioOutputDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return audiodevice.DeviceProperties{
		SampleRate:  d.sampleRate,
		NumChannels: d.numChannels,
	}
}

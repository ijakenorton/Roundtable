package device

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/google/uuid"
)

// --------------------------------------------------------------------------------
// FileAudioInputDevice

// Define an AudioInputDevice that reads from a file in a loop and sends the resulting bytes
// Note that audio file must be a .WAV file.
//
// Also ensure that files match the expected decoding
type FileAudioInputDevice struct {
	logger *slog.Logger
	uuid   uuid.UUID

	shutdownOnce sync.Once

	decoder         *wav.Decoder
	fileHandle      *os.File
	frameDuration   time.Duration
	samplesPerFrame int
	sinkStream      chan frame.PCMFrame
}

// Make a new FileAudioInputDevice from a .WAV file (on the audioFilePath).
//
// The device will play audio from the .WAV file along the channel
// returned by GetStream. The sample rate is determined by the file,
// but the duration between frames is determined by the frameDuration parameter.
//
// Be careful choosing the frameDuration. For example, for OPUS encoding,
// the frameDuration must be one of 2.5, 5, 10, 20, 40, or 60 ms. 20ms is common.
func NewFileAudioInputDevice(
	audioFilePath string,
	frameDuration time.Duration,
) (FileAudioInputDevice, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"file input device uuid", uuid,
	)

	f, err := os.Open(audioFilePath)
	if err != nil {
		logger.Error(
			"could not open audio file",
			"audioFile", audioFilePath,
			"err", err,
		)
		return FileAudioInputDevice{}, err
	}

	decoder := wav.NewDecoder(f)

	if !decoder.IsValidFile() {
		logger.Error(
			"could not decode audio file",
			"audioFile", audioFilePath,
			"err", decoder.Err(),
		)
		return FileAudioInputDevice{}, errors.New("error while decoding audio file")
	}

	samplesPerFrame := int(float64(decoder.NumChans) * float64(decoder.SampleRate) *
		float64(frameDuration) / float64(time.Second))
	if samplesPerFrame <= 0 {
		logger.Error(
			"non-positive samples per frame during opening of file audio input",
			"audioFile", audioFilePath,
			"sampleRate", decoder.SampleRate,
			"channels", decoder.NumChans,
			"samplesPerFrame", samplesPerFrame,
		)
		return FileAudioInputDevice{}, errors.New("non-positive samples per frame")
	}

	logger.Debug(
		"loaded audio file",
		"audioFile", audioFilePath,
		"sampleRate", decoder.SampleRate,
		"channels", decoder.NumChans,
		"samplesPerFrame", samplesPerFrame,
	)

	dataChannel := make(chan frame.PCMFrame)
	return FileAudioInputDevice{
		logger:          logger,
		uuid:            uuid,
		decoder:         decoder,
		fileHandle:      f,
		frameDuration:   frameDuration,
		samplesPerFrame: samplesPerFrame,
		sinkStream:      dataChannel,
	}, nil
}

// Play the audio file loaded by this input device.
// If the context is canceled, the playback stops.
func (d *FileAudioInputDevice) Play(ctx context.Context) {
	d.logger.Debug("playing audio")
	const maxInt16 = float32(math.MaxInt16)
	go func() {
		buf, err := d.decoder.FullPCMBuffer()
		if err != nil {
			slog.Error(
				"could not get full PCM buffer from audio file",
				"err", err,
			)
			return
		}
		frame := make(frame.PCMFrame, d.samplesPerFrame)

		ticker := time.NewTicker(d.frameDuration)
		defer ticker.Stop()
		for frameStart := 0; frameStart < len(buf.Data); frameStart += d.samplesPerFrame {
			frameEnd := min(frameStart+d.samplesPerFrame, len(buf.Data))
			for i := 0; i < frameEnd-frameStart; i += 1 {
				frame[i] = float32(buf.Data[frameStart+i]) / maxInt16
			}

			select {
			case <-ticker.C:
				d.sinkStream <- frame[:frameEnd-frameStart]
			case <-ctx.Done():
				return
			}
		}
		d.logger.Debug("finished playing")
	}()
}

func (d *FileAudioInputDevice) Close() {
	d.logger.Debug("shutdown called")
	d.shutdownOnce.Do(func() {
		close(d.sinkStream)
		d.fileHandle.Close()
	})
}

func (d *FileAudioInputDevice) GetStream() <-chan frame.PCMFrame {
	return d.sinkStream
}

func (d *FileAudioInputDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return audiodevice.DeviceProperties{
		SampleRate:  int(d.decoder.SampleRate),
		NumChannels: int(d.decoder.NumChans),
	}
}

// --------------------------------------------------------------------------------
// FileAudioOutputDevice

// Define an AudioOutputDevice that reads from a channel and writes the result to a .WAV file.
// Note the resulting file is only valid once the input channel is closed.
type FileAudioOutputDevice struct {
	ctx           context.Context
	ctxCancelFunc context.CancelFunc
	logger        *slog.Logger
	uuid          uuid.UUID
	encoder       *wav.Encoder
	fileHandle    *os.File
	sourceStream  <-chan frame.PCMFrame
}

// Create a new FileAudioOutputDevice that writes incoming PCM frames to a .WAV file at the specified path.
func NewFileAudioOutputDevice(
	audioFilePath string,
	sampleRate int,
	numChannels int,
) (FileAudioOutputDevice, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"file input device uuid", uuid,
	)

	f, err := os.Create(audioFilePath)
	if err != nil {
		logger.Error(
			"could not open audio file",
			"audioFile", audioFilePath,
			"err", err,
		)
		return FileAudioOutputDevice{}, err
	}

	encoder := wav.NewEncoder(f, sampleRate, 16, numChannels, 1)

	logger.Debug(
		"loaded audio file",
		"audioFile", audioFilePath,
		"encoder", encoder,
		"sampleRate", encoder.SampleRate,
		"channels", encoder.NumChans,
	)

	dataChannel := make(chan frame.PCMFrame)
	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	return FileAudioOutputDevice{
		ctx:           ctx,
		ctxCancelFunc: ctxCancelFunc,
		logger:        logger,
		uuid:          uuid,
		encoder:       encoder,
		fileHandle:    f,
		sourceStream:  dataChannel,
	}, nil
}

// Wait for this device to be closed
// Blocks until the close function has finished
func (d FileAudioOutputDevice) WaitForClose() {
	<-d.ctx.Done()
}

func (d FileAudioOutputDevice) close() {
	d.encoder.Close()
	d.fileHandle.Sync()
	d.fileHandle.Close()
	d.ctxCancelFunc()
}

// Set the source channel of this audio device, i.e. where data comes from.
// Raw audio data (as PCMFrames) will arrive on the given channel.
//
// When this stream is closed, it is assumed the device will be cleaned up
// (memory will be freed, other channels will be closed, etc)
func (d FileAudioOutputDevice) SetStream(sourceChannel <-chan frame.PCMFrame) {
	d.sourceStream = sourceChannel
	const maxInt16 = float32(math.MaxInt16)
	go func() {
		bufFormat := &goaudio.Format{
			SampleRate:  d.encoder.SampleRate,
			NumChannels: d.encoder.NumChans,
		}
		for pcmFrame := range sourceChannel {
			buf := &goaudio.IntBuffer{
				Format:         bufFormat,
				Data:           make([]int, len(pcmFrame)),
				SourceBitDepth: 16,
			}
			for i, sample := range pcmFrame {
				buf.Data[i] = int(sample * maxInt16)
			}

			err := d.encoder.Write(buf)
			if err != nil {
				d.logger.Error("error while writing frame to file", "err", err)
				continue
			}
		}
		d.logger.Debug("incomingAudio stream closed")
		d.close()
	}()
}

func (d FileAudioOutputDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return audiodevice.DeviceProperties{
		SampleRate:  int(d.encoder.SampleRate),
		NumChannels: int(d.encoder.NumChans),
	}
}

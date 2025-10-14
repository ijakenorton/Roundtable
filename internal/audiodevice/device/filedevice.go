package device

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	goaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/audiodevice"
	"github.com/hmcalister/roundtable/internal/frame"
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
	dataChannel     chan frame.PCMFrame
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
		dataChannel:     dataChannel,
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
				d.dataChannel <- frame[:frameEnd-frameStart]
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
		close(d.dataChannel)
		d.fileHandle.Close()
	})
}

func (d *FileAudioInputDevice) GetStream() <-chan frame.PCMFrame {
	return d.dataChannel
}

func (d *FileAudioInputDevice) GetDeviceProperties() audiodevice.DeviceProperties {
	return audiodevice.DeviceProperties{
		SampleRate:  int(d.decoder.SampleRate),
		NumChannels: int(d.decoder.NumChans),
	}
}


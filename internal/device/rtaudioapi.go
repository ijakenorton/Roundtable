package device

import (
	"log/slog"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/audioapi"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/rtaudiowrapper"
	"github.com/google/uuid"
)

type RtAudioApi struct {
	logger *slog.Logger
	audio  rtaudiowrapper.RtAudio
}

func NewRtAudioApi() (*RtAudioApi, error) {
	uuid := uuid.New()
	logger := slog.Default().With(
		"rtaudio input device uuid", uuid,
	)

	audio, err := rtaudiowrapper.Create(rtaudiowrapper.APIUnspecified)
	if err != nil {
		logger.Error("failed to create rtaudio interface", "err", err)
	}

	return &RtAudioApi{
		logger: logger,
		audio:  audio,
	}, err
}

//Filters RtAudio devices to get only input
func (api *RtAudioApi) InputDevices() []audioapi.AudioIODevice {
	devices, err := api.audio.Devices()
	if err != nil {
		api.logger.Error("Failed to initialise devices")
		return nil
	}

	inputDevices := make([]audioapi.AudioIODevice, 0)

	for _, d := range devices {
		if d.NumInputChannels > 0 {
			inputDevice := audioapi.AudioIODevice{
				ID:   d.ID,
				Name: d.Name,
				DeviceProperties: audiodevice.DeviceProperties{
					SampleRate:  int(d.PreferredSampleRate),
					NumChannels: d.NumInputChannels,
				},
			}
			inputDevices = append(inputDevices, inputDevice)
		}
	}

	return inputDevices
}

//Filters RtAudio devices to get only output
func (api *RtAudioApi) OutputDevices() []audioapi.AudioIODevice {
	devices, err := api.audio.Devices()
	if err != nil {
		api.logger.Error("Failed to initialise devices")
		return nil
	}

	outputDevices := make([]audioapi.AudioIODevice, 0)

	for _, d := range devices {
		if d.NumOutputChannels > 0 {
			outputDevice := audioapi.AudioIODevice{
				ID:   d.ID,
				Name: d.Name,
				DeviceProperties: audiodevice.DeviceProperties{
					SampleRate:  int(d.PreferredSampleRate),
					NumChannels: d.NumOutputChannels,
				},
			}
			outputDevices = append(outputDevices, outputDevice)
		}
	}

	return outputDevices
}

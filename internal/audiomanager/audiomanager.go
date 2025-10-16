package audiomanager

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/frame"
)

// A singleton manager for Audio IO.
// Holds reference to a canonical input/source device (e.g. microphone), a canonical output/sink device (e.g. speakers)
// and handles fan-in / fan-out of audio sources and sinks (giving a copy of audio inputs to each peer).
type AudioManager struct {
	logger *slog.Logger

	// The canonical device to get audio inputs from.
	// Not just *any* producer of audio, but the ultimate source for audio to be forwarded.
	// e.g. a microphone. The source from which all packets flow.
	audioInputDevice audiodevice.AudioSourceDevice

	audioInputSinksMutex sync.RWMutex
	// a list of listeners for new input data.
	// Effectively peers on the network.
	audioInputSinks []chan<- frame.PCMFrame

	// The device to send audio outputs to.
	// Not just *any* consumer of audio, but the ultimate sink for audio to be forwarded to.
	// e.g. a speaker. The sink to which all packets flow.
	audioOutputDevice audiodevice.AudioSinkDevice
}

func NewAudioManager(
	audioInputDevice audiodevice.AudioSourceDevice,
	audioOutputDevice audiodevice.AudioSinkDevice,
	logger *slog.Logger,
) (*AudioManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	manager := &AudioManager{
		logger:            logger,
		audioInputDevice:  audioInputDevice,
		audioInputSinks:   make([]chan<- frame.PCMFrame, 0),
		audioOutputDevice: audioOutputDevice,
	}

	go manager.handleAudioInput()

	return manager, nil
}

// Fan-out the audioInputDevice stream data to all listeners
func (manager *AudioManager) handleAudioInput() {
	go func() {
		inputStream := manager.audioInputDevice.GetStream()
		for data := range inputStream {
			manager.audioInputSinksMutex.Lock()
			// TODO: One channel blocking here will cause all channels to block.
			// Current select approach drops data to listeners who can't accept it... is that fine?
			for _, sink := range manager.audioInputSinks {
				select {
				case sink <- data:
				default:
					// If sink does not accept immediately, drop
					// This means they miss some data... okay?
				}
			}
			manager.audioInputSinksMutex.Unlock()
		}
	}()
}

// Add a new input sink, something that gets a copy of the raw PCM frames of the audioInputDevice.
// The given context is used to signal that the input sink is no longer listening, and will be removed.
//
// Note this function does not guarantee the sink will get every frame from the device, as
// sinks will be skipped if they are not ready to receive the frame immediately.
func (manager *AudioManager) AddInputSink(newSink chan<- frame.PCMFrame, ctx context.Context) {
	manager.audioInputSinksMutex.Lock()
	defer manager.audioInputSinksMutex.Unlock()
	manager.audioInputSinks = append(manager.audioInputSinks, newSink)

	// When the context is canceled, traverse the sink list looking for this sink
	// and remove it by splicing in the last sink and shortening the list.
	go func() {
		<-ctx.Done()
		manager.audioInputSinksMutex.Lock()
		defer manager.audioInputSinksMutex.Unlock()

		close(newSink)
		numSinks := len(manager.audioInputSinks)
		for i, sink := range manager.audioInputSinks {
			if sink == newSink {
				manager.audioInputSinks[i] = manager.audioInputSinks[numSinks-1]
				manager.audioInputSinks = manager.audioInputSinks[:numSinks-1]
				return
			}
		}
	}()

}

// Add a PCM output source, something that can play audio on this client.
// Output sources can play audio by sending raw PCM data along the returned channel.
// The data sent on these channels must in exactly the correct format (matches the properties)
// of the audioOutputDevice
func (manager *AudioManager) AddOutputSource(newSource <-chan frame.PCMFrame) {
	go func() {
		// When dataChannel is closed, we can stop listening on this loop
		for pcmData := range newSource {
			// TODO: Handle audio output reasonably?
			slog.Debug("incoming pcm audio", "pcmData", pcmData)
		}
	}()
}

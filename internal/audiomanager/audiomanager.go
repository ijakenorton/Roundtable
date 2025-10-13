package audiomanager

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/audiodevice"
	"github.com/hmcalister/roundtable/internal/frame"
)

// A singleton manager for Audio IO.
// Holds reference to a input device (e.g. microphone), an output device (e.g. speakers)
// and handles fan-in / fan-out of audio inputs and outputs (giving a copy of audio inputs to each peer)
//
// Also handles the encoding and decoding of raw audio data before transmission.
// Currently, only OPUS is supported for encoding and decoding, with
// sample rate of 48000HZ and mono channels.
type AudioManager struct {
	logger *slog.Logger

	// The device to get audio inputs from
	audioInputDevice audiodevice.AudioInputDevice

	inputListenersMutex sync.RWMutex
	// a list of listeners for new input data.
	// Effectively peers on the network.
	inputListeners []inputListener

	// The device to send audio outputs to
	audioOutputDevice audiodevice.AudioOutputDevice
}

func NewAudioManager(
	audioInputDevice audiodevice.AudioInputDevice,
	audioOutputDevice audiodevice.AudioOutputDevice,
	logger *slog.Logger,
) (*AudioManager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	manager := &AudioManager{
		logger:            logger,
		audioInputDevice:  audioInputDevice,
		inputListeners:    make([]inputListener, 0),
		audioOutputDevice: audioOutputDevice,
	}

	go manager.handleAudioInput()

	return manager, nil
}

// Fan-out the audio input stream data to all listeners
func (manager *AudioManager) handleAudioInput() {
	go func() {
		inputStream := manager.audioInputDevice.GetStream()
		for data := range inputStream {
			manager.inputListenersMutex.Lock()
			// TODO: In a naive world, one channel blocking here will cause all channels to block.
			// Current select approach drops data to listeners who can't accept it... is that fine?
			for _, listener := range manager.inputListeners {
				select {
				case listener.dataChannel <- data:
				default:
					// If listener does not accept immediately, drop
					// This means they miss some data... okay?
				}
			}
			manager.inputListenersMutex.Unlock()
		}
	}()
}

// Add a new input listener, something that gets a copy of the raw PCM frames of the audioInputDevice.
// The given context is used to signal that the input listener is no longer listening, and will be removed.
//
// Note this function does not guarantee the listener will get every frame from the device, as
// listeners will be skipped if they are not ready to receive the frame immediately.
func (manager *AudioManager) AddInputListener(ctx context.Context) <-chan frame.PCMFrame {
	manager.inputListenersMutex.Lock()
	defer manager.inputListenersMutex.Unlock()

	dataChannel := make(chan frame.PCMFrame)
	newListener := inputListener{
		uuid:        uuid.New(),
		dataChannel: dataChannel,
	}
	manager.inputListeners = append(manager.inputListeners, newListener)

	// When the context is canceled, traverse the listener list looking for this listener
	// and remove it by splicing in the last listener and shortening the list.
	go func() {
		<-ctx.Done()
		manager.inputListenersMutex.Lock()
		defer manager.inputListenersMutex.Unlock()

		close(dataChannel)
		numListeners := len(manager.inputListeners)
		for i, listener := range manager.inputListeners {
			if listener.uuid == newListener.uuid {
				manager.inputListeners[i] = manager.inputListeners[numListeners-1]
				manager.inputListeners = manager.inputListeners[:numListeners-1]
				return
			}
		}
	}()

	return dataChannel
}

// Add a PCM output source, something that can play audio on this client.
// Outout sources can play audio by sending raw PCM data along the returned channel.
func (manager *AudioManager) AddOutputSource() chan<- frame.PCMFrame {
	dataChannel := make(chan frame.PCMFrame)
	go func() {
		// When dataChannel is closed, we can stop listening on this loop
		for incomingData := range dataChannel {
			// TODO: Handle audio output reasonably?
			slog.Debug("incoming pcm audio", "incomingData", incomingData)
		}
	}()

	return dataChannel
}

package rtaudio

import (
	"encoding/binary"
	"os"
	"time"
	"fmt"
)
// Speaker plays a WAV file through the default output device
// This is a Go port of the RtAudio playraw.cpp example
func Speaker(wavFilePath string) error {
	// Open the WAV file
	file, err := os.Open(wavFilePath)
	if err != nil {
		return fmt.Errorf("unable to find or open file: %w", err)
	}
	defer file.Close()

	// Read WAV header
	channels, sampleRate, bitsPerSample, dataSize, err := readWavHeader(file)
	if err != nil {
		return fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Verify it's 16-bit format
	if bitsPerSample != 16 {
		return fmt.Errorf("only 16-bit WAV files are currently supported, got %d-bit", bitsPerSample)
	}

	fmt.Printf("Playing WAV file: %s\n", wavFilePath)
	fmt.Printf("Channels: %d, Sample Rate: %d Hz, Bits per Sample: %d\n", channels, sampleRate, bitsPerSample)

	// Create RtAudio instance
	audio, err := Create(APIUnspecified)
	if err != nil {
		return fmt.Errorf("failed to create audio interface: %w", err)
	}
	defer audio.Destroy()

	// Check for available devices
	devices, err := audio.Devices()
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	if len(devices) < 1 {
		return fmt.Errorf("no audio devices found")
	}

	// Calculate total frames
	bytesPerSample := bitsPerSample / 8
	totalFrames := dataSize / (channels * bytesPerSample)

	// Initialize output data
	data := &OutputData{
		file:         file,
		channels:     channels,
		frameCounter: 0,
		totalFrames:  totalFrames,
	}

	// Set up stream parameters for output
	bufferFrames := uint(512)
	params := StreamParams{
		DeviceID:     uint(audio.DefaultOutputDeviceId()),
		NumChannels:  uint(channels),
		FirstChannel: 0,
	}

	// Output callback function
	cb := func(out, in Buffer, dur time.Duration, status StreamStatus) int {
		// Get output buffer as int16 slice
		outputData := out.Int16()
		if outputData == nil {
			return 0
		}

		nFrames := out.Len()

		// Read data from file
		// Note: We read interleaved samples (channels * frames)
		samplesToRead := nFrames * data.channels
		buffer := make([]int16, samplesToRead)

		// Read binary data from file
		err := binary.Read(data.file, binary.LittleEndian, buffer)
		if err != nil {
			if err == os.ErrClosed || err.Error() == "EOF" {
				// End of file - fill with silence and stop
				for i := range outputData {
					outputData[i] = 0
				}
				return 1
			}
			// Other error - fill with silence and stop
			for i := range outputData {
				outputData[i] = 0
			}
			return 1
		}

		// Copy data to output buffer
		copy(outputData, buffer)
		data.frameCounter += nFrames

		// Check for output underflow
		if status&StatusOutputUnderflow != 0 {
			fmt.Println("\nWARNING: Output underflow detected!")
		}

		return 0
	}

	err = audio.Open(&params, nil, FormatInt16, uint(sampleRate), bufferFrames, cb, nil)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}
	defer audio.Close()

	fmt.Printf("Starting playback (buffer frames = %d)...\n", bufferFrames)

	err = audio.Start()
	if err != nil {
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	// Wait for playback to complete
	for audio.IsRunning() {
		time.Sleep(100 * time.Millisecond)
		progress := float64(data.frameCounter) / float64(data.totalFrames) * 100.0
		fmt.Printf("\rProgress: %.1f%% (%d / %d frames)", progress, data.frameCounter, data.totalFrames)
	}

	fmt.Printf("\n\nPlayback complete. Played %d frames.\n", data.frameCounter)

	return nil
}

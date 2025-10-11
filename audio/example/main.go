package main

import (
	"fmt"
	"time"

	"github.com/jake/roundtable/audio"
)

func main() {
	// Create audio device
	dev, err := audio.Create()
	if err != nil {
		fmt.Printf("Failed to create audio device: %v\n", err)
		return
	}
	defer dev.Destroy()

	// List devices
	count := dev.DeviceCount()
	fmt.Printf("Found %d audio devices\n", count)

	defaultInput := dev.GetDefaultInput()
	fmt.Printf("Default input device: %d\n", defaultInput)

	// Start recording: 2 channels, 48kHz, 512 frame buffer
	bufferSize, err := dev.StartRecording(defaultInput, 2, 48000, 512)
	if err != nil {
		fmt.Printf("Failed to start recording: %v\n", err)
		return
	}
	fmt.Printf("Recording started with buffer size: %d frames\n", bufferSize)

	// Record for 5 seconds
	fmt.Println("Recording for 5 seconds...")
	duration := 5 * time.Second
	start := time.Now()

	// Buffer for reading samples (2 channels * 512 frames)
	buffer := make([]int16, 2*512)
	totalFrames := 0

	for time.Since(start) < duration {
		if !dev.IsRunning() {
			fmt.Println("Stream stopped unexpectedly")
			break
		}

		// Read available samples
		framesRead, err := dev.ReadSamples(buffer, 512)
		if err != nil {
			fmt.Printf("Error reading samples: %v\n", err)
			break
		}

		if framesRead > 0 {
			totalFrames += framesRead
		}

		// Don't spin too fast
		time.Sleep(10 * time.Millisecond)
	}

	dev.Stop()
	fmt.Printf("Recording complete. Captured %d frames (%.2f seconds)\n",
		totalFrames, float64(totalFrames)/48000.0)
}

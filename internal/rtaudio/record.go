package rtaudio 
import (
	"log"
	"os"
	"time"
	"fmt"
)

func Record(outpath string) {
	audio, err := Create(APIUnspecified)
	if err != nil {
		log.Fatal(err)
	}
	defer audio.Destroy()
	_, err = audio.Devices()

	devices, err := audio.Devices()

	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	for i := range devices {
		fmt.Printf("%v\n", devices[i])
	}

	// Find the default input device by checking the IsDefaultInput flag
	defaultIn := audio.DefaultInputDevice()

	channels := defaultIn.NumInputChannels
	sampleRate := defaultIn.PreferredSampleRate
	recordTime := 10 // seconds

	// Calculate total frames needed
	totalFrames := int(sampleRate) * recordTime

	// Initialize recording data
	data := &RecordingData{
		buffer:       make([]int16, totalFrames*channels),
		totalFrames:  totalFrames,
		frameCounter: 0,
		channels:     channels,
	}

	params := StreamParams{
		DeviceID:     uint(audio.DefaultInputDeviceId()),
		NumChannels:  uint(channels),
		FirstChannel: 0,
	}

	options := StreamOptions{
		Flags: FlagsScheduleRealtime | FlagsMinimizeLatency,
	}

	// Go callback function that replaces the C input_callback
	cb := func(out, in Buffer, dur time.Duration, status StreamStatus) int {
		// Get input buffer as int16 slice
		inputData := in.Int16()
		if inputData == nil {
			return 0
		}

		nFrames := in.Len()

		// Calculate how many frames to copy
		frames := nFrames
		if data.frameCounter+nFrames > data.totalFrames {
			frames = data.totalFrames - data.frameCounter
		}

		// Copy data to our buffer
		offset := data.frameCounter * data.channels
		samplesToCopy := frames * data.channels
		copy(data.buffer[offset:offset+samplesToCopy], inputData[:samplesToCopy])
		data.frameCounter += frames

		// Return 2 to stop the stream when done
		if data.frameCounter >= data.totalFrames {
			return 2
		}
		return 0
	}

	err = audio.Open(nil, &params, FormatInt16, sampleRate, uint(512), cb, &options)
	if err != nil {
		log.Fatal(err)
	}
	defer audio.Close()

	fmt.Printf("Starting recording for %d seconds...\n", recordTime)

	err = audio.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Wait for recording to complete
	for audio.IsRunning() {
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("\rFrames recorded: %d / %d", data.frameCounter, data.totalFrames)
	}

	fmt.Printf("\n\nRecording complete. Recorded %d frames.\n", data.frameCounter)

	// Write WAV file
	fmt.Printf("Writing WAV file: %s\n", outpath)
	if err := writeWavFile(outpath, data, uint32(sampleRate), 16); err != nil {
		fmt.Printf("Error writing WAV file: %v\n", err)
		return
	}
	fmt.Printf("Successfully wrote %s\n", outpath)
}

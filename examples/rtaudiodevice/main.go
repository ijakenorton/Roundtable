package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Honorable-Knights-of-the-Roundtable/rtaudiowrapper"
)

func main() {
	// Define command-line flags
	mode := flag.String("mode", "record", "Mode: 'record' or 'play'")
	file := flag.String("file", "./assets/media.wav", "WAV file path")

	flag.Parse()
	fmt.Printf("File %s\n", *file)

	dir := filepath.Dir(*file)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	switch *mode {
	case "record":
		fmt.Printf("Recording to: %s\n", *file)
		rtaudiowrapper.Record(*file)
	case "play":
		fmt.Printf("Playing from: %s\n", *file)
		if err := rtaudiowrapper.Speaker(*file); err != nil {
			fmt.Fprintf(os.Stderr, "Error playing file: %v\n", err)
			os.Exit(1)
		}
	case "devices":
		audio, err := rtaudiowrapper.Create(rtaudiowrapper.APIUnspecified)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create rtaudio device\n")
		}

		devices, err := audio.Devices()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err)
		}

		for _, device := range devices {
			fmt.Printf("%v\n", device)
		}

	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Use 'record' or 'play'\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}

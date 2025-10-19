package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/rtaudio"
)

func main() {
	// Define command-line flags
	mode := flag.String("mode", "record", "Mode: 'record' or 'play'")
	file := flag.String("file", "./assets/recording.wav", "WAV file path")
	flag.Parse()

	switch *mode {
	case "record":
		fmt.Printf("Recording to: %s\n", *file)
		rtaudio.Record(*file)
	case "play":
		fmt.Printf("Playing from: %s\n", *file)
		if err := rtaudio.Speaker(*file); err != nil {
			fmt.Fprintf(os.Stderr, "Error playing file: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s. Use 'record' or 'play'\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}

//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const (
	output  = "rtaudio_go.o"
	cppFile = "./lib/rtaudio_go.cpp"
)

func main() {
	if err := build(); err != nil {
		fatal("Build failed: %v", err)
	}
	fmt.Println("Build successful!")
}

func build() error {
	switch runtime.GOOS {
	case "windows":
		return buildWindows()
	case "linux":
		return buildLinux()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func buildWindows() error {
	fmt.Println("Building for Windows with WASAPI...")

	winLibs := []string{
		"-lole32",
		"-lwinmm",
		"-lksuser",
		"-lmfplat",
		"-lmfuuid",
		"-lwmcodecdspuuid",
	}

	args := []string{
		"-c",
		"-o", output,
		cppFile,
		"-D__WINDOWS_WASAPI__",
	}
	args = append(args, winLibs...)

	return runCommand("g++", args...)
}

func buildLinux() error {
	fmt.Println("Building for Linux with JACK...")

	args := []string{
		"-c",
		"-o", output,
		cppFile,
		"-D__UNIX_JACK__",
		"-lpthread",
		"-lm",
		"-ljack",
		"-lstdc++",
	}

	return runCommand("g++", args...)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Running: %s %v\n", name, args)

	return cmd.Run()
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

# RtAudio Device Example

This example demonstrates recording and playback functionality using RtAudio.

Ensure that the root Makefile has been executed appropriately before running these examples. From the root of this repo, run: `make git_submodule_init git_submodule_build`.

# Usage

### Recording Audio

Record audio from the default input device to a WAV file:

```bash
go run main.go -mode=record -file=./myrecording.wav
```

This will record 10 seconds of audio to the specified file.

### Playing Audio

Play a WAV file through the default output device:

```bash
go run main.go -mode=play -file=./assets/recording.wav
```

**Note:** Currently only 16-bit PCM WAV files are supported for playback.

## Default Behavior

If no flags are specified, the program defaults to recording mode with output file `./assets/recording.wav`:

```bash
go run main.go
```

## Testing the Speaker Output

A quick way to test the speaker output is to first record some audio, then play it back:

```bash
# Record 10 seconds
go run main.go -mode=record -file=test.wav

# Play it back
go run main.go -mode=play -file=test.wav
```

You can also use any of the existing WAV files in the `assets/` directory for testing playback.

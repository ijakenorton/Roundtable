package audiomanager

import (
	"github.com/hmcalister/roundtable/internal/audiodevice"
	"github.com/hmcalister/roundtable/internal/frame"
)

// Middle-man processing for incoming audio data,
// to match the incoming data format to the expected output format.
// Data on the given input channel should be in the specified input format.
// Data on the returned output channel will be in the specified output format.
//
// e.g. if the incoming format is mono, but the output device expects stereo,
// a processing stream will handle that conversion.
//
// When the input channel is closed, the output channel is also closed.
func newProcessingStream(
	inputChannel <-chan frame.PCMFrame,
	inputProperties audiodevice.DeviceProperties,
	outputProperties audiodevice.DeviceProperties,
) chan<- frame.PCMFrame {
	outputChannel := make(chan frame.PCMFrame)
	processingFunctions := make([]outputProcessingFunction, 0)

	if inputProperties.NumChannels == 1 && outputProperties.NumChannels == 2 {
		processingFunctions = append(processingFunctions, monoToStereo)
	}
	if inputProperties.NumChannels == 2 && outputProperties.NumChannels == 1 {
		processingFunctions = append(processingFunctions, stereoToMono)
	}
	if inputProperties.SampleRate != outputProperties.SampleRate {
		processingFunctions = append(processingFunctions, newResampleFunction(outputProperties.SampleRate))
	}

	go func() {
		for frame := range inputChannel {
			for _, f := range processingFunctions {
				frame = f(frame)
			}
			outputChannel <- frame
		}

		close(outputChannel)
	}()

	return outputChannel
}

// --------------------------------------------------------------------------------

// Stand in for any function that processes the output in some way.
type outputProcessingFunction func(frame.PCMFrame) frame.PCMFrame

func monoToStereo(input frame.PCMFrame) frame.PCMFrame {
	// TODO
	return input
}

func stereoToMono(input frame.PCMFrame) frame.PCMFrame {
	// TODO
	return input
}

func newResampleFunction(newSampleRate int) outputProcessingFunction {
	return func(input frame.PCMFrame) frame.PCMFrame {
		// TODO
		return input
	}
}

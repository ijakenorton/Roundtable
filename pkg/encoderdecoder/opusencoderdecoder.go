package encoderdecoder

import (
	"errors"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
	"github.com/jj11hh/opus"
)

// Valid frame durations for OPUS encoding
//
// Longer frame durations introduce more latency, but are more bandwidth-efficient and potentially higher quality
type OPUSFrameDuration time.Duration

const (
	OPUS_FRAME_DURATION_2_POINT_5_MS OPUSFrameDuration = OPUSFrameDuration(2500 * time.Microsecond)
	OPUS_FRAME_DURATION_5_MS         OPUSFrameDuration = OPUSFrameDuration(5 * time.Millisecond)
	OPUS_FRAME_DURATION_10_MS        OPUSFrameDuration = OPUSFrameDuration(10 * time.Millisecond)
	OPUS_FRAME_DURATION_20_MS        OPUSFrameDuration = OPUSFrameDuration(20 * time.Millisecond)
	OPUS_FRAME_DURATION_40_MS        OPUSFrameDuration = OPUSFrameDuration(40 * time.Millisecond)
	OPUS_FRAME_DURATION_60_MS        OPUSFrameDuration = OPUSFrameDuration(60 * time.Millisecond)
	OPUS_FRAME_DURATION_120_MS       OPUSFrameDuration = OPUSFrameDuration(120 * time.Millisecond)
)

type OpusEncoderDecoder struct {
	sampleRate  int
	numChannels int

	encoder       *opus.Encoder
	encodingFrame frame.EncodedFrame
	decoder       *opus.Decoder
	decodedFrame  frame.PCMFrame
}

func newOpusEncoderDecoder(sampleRate int, numChannels int) (OpusEncoderDecoder, error) {
	encoder, errEnc := opus.NewEncoder(sampleRate, numChannels, opus.Application(opus.AppVoIP))
	decoder, errDec := opus.NewDecoder(sampleRate, numChannels)
	if err := errors.Join(errEnc, errDec); err != nil {
		return OpusEncoderDecoder{}, err
	}

	bufferSize := sampleRate * numChannels * 20 * 5 / 1000 // Register enough space to hold 5 frames
	return OpusEncoderDecoder{
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		encoder:       encoder,
		encodingFrame: make(frame.EncodedFrame, bufferSize),
		decoder:       decoder,
		decodedFrame:  make(frame.PCMFrame, bufferSize),
	}, nil
}

// TODO:
// What if the incoming frame is *not* a magic number?
func (encdec OpusEncoderDecoder) Encode(pcmData frame.PCMFrame) (frame.EncodedFrame, error) {

	encodedBytes, err := encdec.encoder.EncodeFloat32(pcmData, encdec.encodingFrame)
	if err != nil {
		return nil, err
	}
	return encdec.encodingFrame[:encodedBytes], nil
}

func (encdec OpusEncoderDecoder) Decode(encodedData frame.EncodedFrame) (frame.PCMFrame, error) {
	decodedBytes, err := encdec.decoder.DecodeFloat32(encodedData, encdec.decodedFrame)
	if err != nil {
		return nil, err
	}
	return encdec.decodedFrame[:decodedBytes*encdec.numChannels], nil
}

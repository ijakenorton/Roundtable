package encoderdecoder

import (
	"errors"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/frame"
	"github.com/hraban/opus"
)

const (
	bufferSize = 8192
)

// TODO:
// Is it okay to have a static frame for encoding and decoding into?
// Will this get overwritten before being consumed?
// Better to have a ring buffer of frames to use?

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

	return OpusEncoderDecoder{
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		encoder:       encoder,
		encodingFrame: make(frame.EncodedFrame, bufferSize),
		decoder:       decoder,
		decodedFrame:  make(frame.PCMFrame, bufferSize),
	}, nil
}

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

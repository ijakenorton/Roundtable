package encoderdecoder

import (
	"errors"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
)

type EncoderDecoderTypeEnum string

var (
	EncoderDecoderTypeNotImplemented EncoderDecoderTypeEnum = "not implemented"
	EncoderDecoderTypeNull           EncoderDecoderTypeEnum = "null"
	EncoderDecoderTypeOpus           EncoderDecoderTypeEnum = "opus"
)

var (
	errEncoderDecoderTypeNotImplemented = errors.New("specified encoderdecoder type is not implemented")
)

// Audio encoder/decoder interface.
// Used to encode raw PCM Frames to an encoded frame,
// and decode those frames back to PCM frames
type EncoderDecoder interface {
	Encode(pcmData frame.PCMFrame) (frame.EncodedFrame, error)
	Decode(encodedData frame.EncodedFrame) (frame.PCMFrame, error)
}

// Create a new encoder/decoder based on the negotiated codec
// If something goes wrong during creation of an encoder/decoder
// (e.g. the mime type does not have an implementation) then a nil Encoder/Decoder
// and an error is returned.
func NewEncoderDecoder(
	encoderdecoderID EncoderDecoderTypeEnum,
	sampleRate int,
	numChannels int,

) (EncoderDecoder, error) {
	switch encoderdecoderID {
	case EncoderDecoderTypeNull:
		return NullEncoderDecoder{}, nil
	case EncoderDecoderTypeOpus:
		return newOpusEncoderDecoder(sampleRate, numChannels)
	case EncoderDecoderTypeNotImplemented:
		return nil, errEncoderDecoderTypeNotImplemented
	default:
		return nil, errEncoderDecoderTypeNotImplemented
	}
}

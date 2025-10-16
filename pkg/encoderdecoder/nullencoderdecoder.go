package encoderdecoder

import (
	"errors"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/frame"
)

var (
	errNullEncoderDecoderUsed error = errors.New("null encoder decoder used")
)

// An encoder decoder that does NO ENCODING/DECODING
// Instead, an error is *always* returned.
//
// This is not the NoneEncoderDecoder, i.e. converting PCMFrames to EncodedFrames
// directly without compression, the NullEncoderDecoder throws away
// any PCMFrames or EncodedFrames and returns errors every time
type NullEncoderDecoder struct{}

func (encdec NullEncoderDecoder) Encode(_ frame.PCMFrame) (frame.EncodedFrame, error) {
	return nil, errNullEncoderDecoderUsed
}

func (encdec NullEncoderDecoder) Decode(_ frame.EncodedFrame) (frame.PCMFrame, error) {
	return nil, errNullEncoderDecoderUsed
}

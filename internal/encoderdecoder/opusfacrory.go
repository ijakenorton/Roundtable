package encoderdecoder

import (
	"fmt"
	"time"
)

type OpusFactory struct {
	frameDuration OPUSFrameDuration
}

func NewOpusFactor(
	frameDuration time.Duration,
) (OpusFactory, error) {
	opusFrameDuration := OPUSFrameDuration(frameDuration)
	switch opusFrameDuration {
	case OPUS_FRAME_DURATION_2_POINT_5_MS:
	case OPUS_FRAME_DURATION_5_MS:
	case OPUS_FRAME_DURATION_10_MS:
	case OPUS_FRAME_DURATION_20_MS:
	case OPUS_FRAME_DURATION_40_MS:
	case OPUS_FRAME_DURATION_60_MS:
	case OPUS_FRAME_DURATION_120_MS:
	default:
		return OpusFactory{}, fmt.Errorf("frame duration %v is not a valid OPUS frame duration", frameDuration)
	}

	return OpusFactory{
		frameDuration: opusFrameDuration,
	}, nil
}

func (f OpusFactory) NewOpusEncoderDecoder(sampleRate int, numChannels int) (*OpusEncoderDecoder, error) {
	return newOpusEncoderDecoder(sampleRate, numChannels, f.frameDuration)
}

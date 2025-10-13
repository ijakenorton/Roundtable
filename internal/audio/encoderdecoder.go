package audio

// Audio encoder/decoder interface.
// Used to encode raw PCM Frames to an encoded frame,
// and decode those frames back to PCM frames
type EncoderDecoder interface {
	Encode([]PCMFrame) []EncodedFrame
	Decode([]EncodedFrame) []PCMFrame
}

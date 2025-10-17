package encoderdecoder

import "time"

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

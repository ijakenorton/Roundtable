package networking

import "github.com/pion/webrtc/v4"

var (
	// Define a mapping from string representation (e.g. for use in config files) to codec specification
	CodecMap map[string]webrtc.RTPCodecCapability = map[string]webrtc.RTPCodecCapability{
		"CodecOpus48000Stereo": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		"CodecOpus48000Mono": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  1,
		},
	}
)

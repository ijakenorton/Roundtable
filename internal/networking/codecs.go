package networking

import "github.com/pion/webrtc/v4"

var (
	CodecOpus48000Stereo webrtc.RTPCodecCapability = webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeOpus,
		ClockRate: 48000,
		Channels:  2,
	}

	CodecOpus48000Mono webrtc.RTPCodecCapability = webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeOpus,
		ClockRate: 48000,
		Channels:  1,
	}
)

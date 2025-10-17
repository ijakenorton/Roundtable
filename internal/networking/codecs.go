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
		"CodecOpus24000Stereo": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 24000,
			Channels:  2,
		},
		"CodecOpus24000Mono": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 24000,
			Channels:  1,
		},
		"CodecOpus16000Stereo": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 16000,
			Channels:  2,
		},
		"CodecOpus16000Mono": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 16000,
			Channels:  1,
		},
		"CodecOpus12000Stereo": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 12000,
			Channels:  2,
		},
		"CodecOpus12000Mono": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 12000,
			Channels:  1,
		},
		"CodecOpus8000Stereo": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 8000,
			Channels:  2,
		},
		"CodecOpus8000Mono": {
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 8000,
			Channels:  1,
		},
	}
)

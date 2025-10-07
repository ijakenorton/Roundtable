package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/hmcalister/roundtable/cmd/config"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/spf13/viper"
)

func main() {
	configFilePath := flag.String("configFilePath", "config.yaml", "Set the file path to the config file.")
	flag.Parse()

	config.LoadConfig(*configFilePath)
	logFilePointer := config.ConfigureLogger()
	if logFilePointer != nil {
		defer logFilePointer.Close()
	}

	// --------------------------------------------------------------------------------

	webrtcServer := webrtc.ICEServer{
		URLs: viper.GetStringSlice("ICEServers"),
	}

	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{webrtcServer},
	}

	peerOne, errOne := webrtc.NewPeerConnection(webrtcConfig)
	peerTwo, errTwo := webrtc.NewPeerConnection(webrtcConfig)
	if err := errors.Join(errOne, errTwo); err != nil {
		slog.Error("error when creating peer connection",
			"err", err,
			"webrtcConfig", webrtcConfig,
		)
		panic(err)
	}

	// --------------------------------------------------------------------------------

	peerOne.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		slog.Info(
			"peer one connection state change",
			"peer connection state", pcs.String(),
		)
	})

	peerTwo.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		slog.Info(
			"peer two connection state change",
			"peer connection state", pcs.String(),
		)
	})

	// --------------------------------------------------------------------------------

	// Create audio track for peer one (sender) - using PCM
	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU}, // PCM μ-law
		"audio",
		"microphone",
	)
	if err != nil {
		slog.Error("error creating audio track", "err", err)
		panic(err)
	}

	// Add audio track to peer one
	rtpSender, err := peerOne.AddTrack(audioTrack)
	if err != nil {
		slog.Error("error adding audio track", "err", err)
		panic(err)
	}

	// Handle RTCP packets for the audio track
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Set up audio receiver on peer two
	peerTwo.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		slog.Info("Received audio track",
			"id", track.ID(),
			"kind", track.Kind().String(),
			"mime", track.Codec().MimeType,
		)

		// Create WAV file for recording received audio
		wavFile, err := os.Create("received_audio.wav")
		if err != nil {
			slog.Error("Error creating WAV file", "err", err)
			return
		}

		// Write WAV header (will update later with correct size)
		writeWAVHeader(wavFile, 8000, 1, 16) // 8kHz, mono, 16-bit

		var totalSamples uint32
		defer func() {
			// Sync file to ensure all data is written
			wavFile.Sync()
			// Update WAV header with correct file size
			updateWAVHeader(wavFile, totalSamples)
			wavFile.Sync()
			wavFile.Close()
			slog.Info("Audio recording saved", "file", "received_audio.wav", "samples", totalSamples, "duration_seconds", float64(totalSamples)/8000.0)
		}()

		// Read RTP packets from the track
		go func() {
			for {
				rtp, _, err := track.ReadRTP()
				if err != nil {
					slog.Error("Error reading RTP packet", "err", err)
					return
				}

				slog.Debug("Received audio RTP packet", "payloadLen", len(rtp.Payload))

				// Decode μ-law payload to 16-bit PCM
				pcmData := make([]int16, len(rtp.Payload))
				for i, mulawByte := range rtp.Payload {
					pcmData[i] = mulawToLinear(mulawByte)
				}

				// Write PCM data to WAV file
				for _, sample := range pcmData {
					binary.Write(wavFile, binary.LittleEndian, sample)
				}
				totalSamples += uint32(len(pcmData))

				// Log periodically
				if totalSamples%8000 == 0 { // Every second of audio
					slog.Info("Recorded audio", "seconds", totalSamples/8000)
				}
			}
		}()
	})

	// --------------------------------------------------------------------------------

	dataChannelOptions := &webrtc.DataChannelInit{}

	dataChannel, err := peerOne.CreateDataChannel("pingChannel", dataChannelOptions)
	if err != nil {
		slog.Error("error when creating data channel",
			"err", err,
			"dataChannelOptions", dataChannelOptions,
		)
		panic(err)
	}

	dataChannel.OnOpen(func() {
		slog.Info("peer one opened data channel")
		go func() {
			for i := 0; ; i += 1 {
				msg := fmt.Sprintf("Ping %d", i)
				slog.Info("peer one sending ping", "msg", msg)
				if err := dataChannel.SendText(msg); err != nil {
					slog.Error("error when sending ping", "err", err)
				}
				time.Sleep(time.Second)
			}
		}()
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		slog.Info("peer one received", "message", string(msg.Data))
	})

	peerTwo.OnDataChannel(func(dc *webrtc.DataChannel) {
		slog.Info("peer two received data channel",
			"data channel label", dc.Label(),
			"data channel ID", dc.ID(),
		)

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			slog.Info("peer two received", "msg", string(msg.Data))
			reply := fmt.Sprintf("pong %s", string(msg.Data))
			slog.Info("peer two sending", "reply", reply)
			dc.SendText(reply)
		})
	})

	// --------------------------------------------------------------------------------

	offerOptions := &webrtc.OfferOptions{}
	answerOptions := &webrtc.AnswerOptions{}

	slog.Info("creating offer")
	offer, err := peerOne.CreateOffer(offerOptions)
	if err != nil {
		slog.Error(
			"error when creating offer",
			"err", err,
			"offerOptions", offerOptions,
		)
		panic(err)
	}

	if err = peerOne.SetLocalDescription(offer); err != nil {
		slog.Error(
			"error when setting local description of offer",
			"err", err,
			"offer", offer,
		)
		panic(err)
	}

	// Set up ICE candidate trickling
	peerOne.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			slog.Info("Peer one found ICE candidate", "candidate", candidate.String())
			// Directly add to peer two since they're in the same process
			if err := peerTwo.AddICECandidate(candidate.ToJSON()); err != nil {
				slog.Error("Error adding ICE candidate to peer two", "err", err)
			}
		}
	})

	if err = peerTwo.SetRemoteDescription(*peerOne.LocalDescription()); err != nil {
		slog.Error(
			"error when setting remote description of offer",
			"err", err,
		)
		panic(err)
	}

	slog.Info("creating answer")
	answer, err := peerTwo.CreateAnswer(answerOptions)
	if err != nil {
		slog.Error(
			"error when creating answer",
			"err", err,
			"answerOptions", answerOptions,
		)
		panic(err)
	}

	if err = peerTwo.SetLocalDescription(answer); err != nil {
		slog.Error(
			"error when setting local description of answer",
			"err", err,
			"answer", answer,
		)
		panic(err)
	}

	// Set up ICE candidate trickling for peer two
	peerTwo.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			slog.Info("Peer two found ICE candidate", "candidate", candidate.String())
			// Directly add to peer one since they're in the same process
			if err := peerOne.AddICECandidate(candidate.ToJSON()); err != nil {
				slog.Error("Error adding ICE candidate to peer one", "err", err)
			}
		}
	})

	if err = peerOne.SetRemoteDescription(*peerTwo.LocalDescription()); err != nil {
		slog.Error(
			"error when setting remote description of answer",
			"err", err,
		)
		panic(err)
	}

	// --------------------------------------------------------------------------------

	// Start simulated audio generation
	slog.Info("Starting simulated audio generation")

	// Audio parameters - PCM μ-law uses 8kHz sample rate
	const sampleRate = 8000
	const channels = 1
	const framesPerBuffer = 160 // 20ms at 8kHz

	// Audio generation state
	var phase float64
	const baseFreq = 440.0 // A4 note
	var frameCount int64

	// Start audio generation goroutine
	go func() {
		ticker := time.NewTicker(time.Millisecond * 20) // 20ms frames
		defer ticker.Stop()

		for range ticker.C {
			// Generate simulated audio samples
			pcm := make([]int16, framesPerBuffer)

			for i := 0; i < framesPerBuffer; i++ {
				// Create a more interesting waveform: sine wave with slow frequency modulation
				freqModulation := math.Sin(float64(frameCount)*0.01) * 50 // ±50Hz modulation (reduced for 8kHz)
				currentFreq := baseFreq + freqModulation

				// Generate sine wave sample
				sample := math.Sin(phase)

				// Add some harmonics for richer sound (but keep under Nyquist frequency)
				if currentFreq*2 < sampleRate/2 {
					sample += 0.3 * math.Sin(phase*2) // Second harmonic
				}
				if currentFreq*3 < sampleRate/2 {
					sample += 0.1 * math.Sin(phase*3) // Third harmonic
				}

				// Apply some amplitude modulation for "breathing" effect
				amplitude := 0.3 + 0.2*math.Sin(float64(frameCount)*0.005)
				sample *= amplitude

				// Convert to int16
				pcm[i] = int16(sample * 16383) // Use 50% of max volume for safety

				// Update phase
				phase += 2 * math.Pi * currentFreq / sampleRate
				if phase >= 2*math.Pi {
					phase -= 2 * math.Pi
				}
			}

			frameCount++

			// Convert PCM to μ-law for PCMU codec
			mulawData := make([]byte, framesPerBuffer)
			for i, sample := range pcm {
				mulawData[i] = linearToMulaw(sample)
			}

			// Send to WebRTC track
			if err := audioTrack.WriteSample(media.Sample{
				Data:     mulawData,
				Duration: time.Millisecond * 20, // 20ms frame
			}); err != nil {
				slog.Error("error writing audio sample", "err", err)
			}

			// Log periodically
			if frameCount%250 == 0 { // Every 5 seconds
				slog.Info("Generated audio frames", "frameCount", frameCount, "frequency", baseFreq+math.Sin(float64(frameCount)*0.01)*50)
			}
		}
	}()

	slog.Info("Simulated audio generation started - generating test tones!")

	// Keep process alive for pings and audio to pass
	select {}
}

// linearToMulaw converts a 16-bit linear PCM sample to μ-law encoding
func linearToMulaw(sample int16) byte {
	// μ-law encoding algorithm
	const bias = 0x84
	const clip = 0x7F7F

	var sign byte
	var exponent byte
	var mantissa byte

	// Get the sign bit
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}

	// Clip the sample
	if sample > clip {
		sample = clip
	}

	// Add bias and work with uint16 to avoid overflow
	biasedSample := uint16(sample) + bias

	// Find the exponent
	if biasedSample >= 0x8000 {
		exponent = 7
	} else if biasedSample >= 0x4000 {
		exponent = 6
	} else if biasedSample >= 0x2000 {
		exponent = 5
	} else if biasedSample >= 0x1000 {
		exponent = 4
	} else if biasedSample >= 0x0800 {
		exponent = 3
	} else if biasedSample >= 0x0400 {
		exponent = 2
	} else if biasedSample >= 0x0200 {
		exponent = 1
	} else {
		exponent = 0
	}

	// Get the mantissa
	mantissa = byte((biasedSample >> (exponent + 3)) & 0x0F)

	// Combine sign, exponent, and mantissa
	return sign | (exponent << 4) | mantissa
}

// mulawToLinear converts a μ-law encoded byte to 16-bit linear PCM
func mulawToLinear(mulawByte byte) int16 {
	// μ-law decoding lookup table
	mulawTable := [256]int16{
		-32124, -31100, -30076, -29052, -28028, -27004, -25980, -24956,
		-23932, -22908, -21884, -20860, -19836, -18812, -17788, -16764,
		-15996, -15484, -14972, -14460, -13948, -13436, -12924, -12412,
		-11900, -11388, -10876, -10364, -9852, -9340, -8828, -8316,
		-7932, -7676, -7420, -7164, -6908, -6652, -6396, -6140,
		-5884, -5628, -5372, -5116, -4860, -4604, -4348, -4092,
		-3900, -3772, -3644, -3516, -3388, -3260, -3132, -3004,
		-2876, -2748, -2620, -2492, -2364, -2236, -2108, -1980,
		-1884, -1820, -1756, -1692, -1628, -1564, -1500, -1436,
		-1372, -1308, -1244, -1180, -1116, -1052, -988, -924,
		-876, -844, -812, -780, -748, -716, -684, -652,
		-620, -588, -556, -524, -492, -460, -428, -396,
		-372, -356, -340, -324, -308, -292, -276, -260,
		-244, -228, -212, -196, -180, -164, -148, -132,
		-120, -112, -104, -96, -88, -80, -72, -64,
		-56, -48, -40, -32, -24, -16, -8, 0,
		32124, 31100, 30076, 29052, 28028, 27004, 25980, 24956,
		23932, 22908, 21884, 20860, 19836, 18812, 17788, 16764,
		15996, 15484, 14972, 14460, 13948, 13436, 12924, 12412,
		11900, 11388, 10876, 10364, 9852, 9340, 8828, 8316,
		7932, 7676, 7420, 7164, 6908, 6652, 6396, 6140,
		5884, 5628, 5372, 5116, 4860, 4604, 4348, 4092,
		3900, 3772, 3644, 3516, 3388, 3260, 3132, 3004,
		2876, 2748, 2620, 2492, 2364, 2236, 2108, 1980,
		1884, 1820, 1756, 1692, 1628, 1564, 1500, 1436,
		1372, 1308, 1244, 1180, 1116, 1052, 988, 924,
		876, 844, 812, 780, 748, 716, 684, 652,
		620, 588, 556, 524, 492, 460, 428, 396,
		372, 356, 340, 324, 308, 292, 276, 260,
		244, 228, 212, 196, 180, 164, 148, 132,
		120, 112, 104, 96, 88, 80, 72, 64,
		56, 48, 40, 32, 24, 16, 8, 0,
	}
	return mulawTable[mulawByte]
}

// writeWAVHeader writes a WAV file header
func writeWAVHeader(file *os.File, sampleRate, channels, bitsPerSample uint32) {
	// RIFF header
	file.WriteString("RIFF")
	binary.Write(file, binary.LittleEndian, uint32(0)) // File size (will update later)
	file.WriteString("WAVE")

	// Format chunk
	file.WriteString("fmt ")
	binary.Write(file, binary.LittleEndian, uint32(16))                          // Subchunk1Size (16 for PCM)
	binary.Write(file, binary.LittleEndian, uint16(1))                           // AudioFormat (1 = PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))                    // NumChannels
	binary.Write(file, binary.LittleEndian, sampleRate)                          // SampleRate
	binary.Write(file, binary.LittleEndian, sampleRate*channels*bitsPerSample/8) // ByteRate
	binary.Write(file, binary.LittleEndian, uint16(channels*bitsPerSample/8))    // BlockAlign
	binary.Write(file, binary.LittleEndian, uint16(bitsPerSample))               // BitsPerSample

	// Data chunk header
	file.WriteString("data")
	binary.Write(file, binary.LittleEndian, uint32(0)) // Subchunk2Size (will update later)
}

// updateWAVHeader updates the WAV file header with the correct file sizes
func updateWAVHeader(file *os.File, totalSamples uint32) {
	dataSize := totalSamples * 2 // 2 bytes per sample for 16-bit
	fileSize := dataSize + 36    // 36 bytes for header (minus 8 for RIFF chunk header)

	// Update file size in RIFF header (at offset 4) - this should be total file size minus 8
	file.Seek(4, 0)
	binary.Write(file, binary.LittleEndian, fileSize)

	// Update data size in data chunk header (at offset 40)
	file.Seek(40, 0)
	binary.Write(file, binary.LittleEndian, dataSize)

	// Ensure we're back at the end of file
	file.Seek(0, 2)
}

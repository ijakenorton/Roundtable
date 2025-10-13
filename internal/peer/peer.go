package peer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/encoderdecoder"
	"github.com/hmcalister/roundtable/internal/frame"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const (
	HEARTBEAT_PERIOD time.Duration = 10 * time.Second
)

// A Peer object should not be constructed directly.
// Instead, Peers should be made using a PeerFactory and the corresponding NewOfferingPeer and NewAnsweringPeer methods.
//
// Even then, these methods are likely never going to be called explicitly by the main client, and
// Peers will instead be constructed by a WebRTCConnectionManager using Dial (returning a Peer directly)
// or listenIncomingSessionOffers (returning a Peer along the connection manager's) IncomingConnectionChannel channel.
type Peer struct {
	logger *slog.Logger
	uuid   uuid.UUID

	// --------------------------------------------------------------------------------
	// Connection related fields

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection

	// WebRTC track for sending audio from this client to the remote client
	connectionAudioInputTrack *webrtc.TrackLocalStaticSample

	// WebRTC track for receiving audio from remote client
	connectionAudioOutputTrack *webrtc.TrackRemote

	// Data Channel to send / receive heartbeat messages on
	connectionHeartbeatDataChannel *webrtc.DataChannel

	// --------------------------------------------------------------------------------
	// Audio Input / Output fields

	// Audio input data from this client is passed in on this channel to be sent to remote peers.
	audioInputChannel <-chan frame.PCMFrame

	// Function to signal the closing of the audioInputChannel, meaning no more data is to be sent along it.
	audioInputChannelCancelFunc context.CancelFunc

	// Audio output data from this client is passed along this channel to be played on the audio output device.
	audioOutputChannel chan<- frame.PCMFrame

	// audioOutputChannel context, to signal the peer should stop listening for incoming data
	audioOutputChannelCancelFunc context.CancelFunc

	// audioOutputChannel waitgroup, to ensure the receiveAudioOutputHandler go routine finishes
	audioOutputChannelWaitGroup sync.WaitGroup

	// Audio encoder / decoder to be used for this connection only
	audioEncoderDecoder encoderdecoder.EncoderDecoder
}

// Set the audioInputChannel of this peer.
// The given channel should stream raw PCM frames from this clients audio input device (e.g. microphone)/
//
// When this peer is shutdown, the given cancel function is called to signal no more data is to be sent on the channel.
func (peer *Peer) SetAudioInputChannel(c <-chan frame.PCMFrame, cancel context.CancelFunc) {
	peer.audioInputChannel = c
	peer.audioInputChannelCancelFunc = cancel
}

// Set the audioOutputChannel of this peer.
// The given channel should accept raw PCM frames to be played on this clients audio output device.
// The source of these frames is the audio input device of the remote peer.
//
// When this peer is shutdown, the given channel is closed (hence, no data is to be sent on it anymore)
func (peer *Peer) SetAudioOutputChannel(c chan<- frame.PCMFrame) {
	peer.audioOutputChannel = c
}

func (peer *Peer) setConnectionHeartbeatDataChannel(dc *webrtc.DataChannel) {
	peer.connectionHeartbeatDataChannel = dc
	dc.OnOpen(peer.heartbeatOnOpenHandler)
	dc.OnMessage(peer.heartbeatOnMessageHandler)
}

func (peer *Peer) setConnectionAudioInputTrack(tr *webrtc.TrackLocalStaticSample) {
	peer.connectionAudioInputTrack = tr
}

func (peer *Peer) gracefulShutdown() {
	peer.connection.Close()
	peer.audioInputChannelCancelFunc()
	peer.audioInputChannelCancelFunc()
	peer.audioOutputChannelWaitGroup.Wait()
	close(peer.audioOutputChannel)
}

// --------------------------------------------------------------------------------
// CONNECTION HANDLERS
// Handlers for various aspects of the PeerConnection

// OnConnectionStateChange handler
// Handles changes of state on the connection, such as connection establishment and graceful shutdown
func (peer *Peer) onConnectionStateChangeHandler(pcs webrtc.PeerConnectionState) {
	peer.logger.Debug("peer connection state change", "new state", pcs.String())
	switch pcs {
	case webrtc.PeerConnectionStateConnected:
		peer.logger.Info("peer connection connected")
		peer.connectionConnectedHandler()

	case webrtc.PeerConnectionStateFailed:
		peer.logger.Info("peer connection failed")
		// TODO: Handle failed connection
		peer.gracefulShutdown()

	case webrtc.PeerConnectionStateDisconnected:
		peer.logger.Info("peer connection disconnected")
		// TODO: Handle disconnected connection
		peer.gracefulShutdown()

	case webrtc.PeerConnectionStateClosed:
		peer.logger.Info("peer connection closed")
		// TODO: Handle closed connection
		peer.gracefulShutdown()
	}
}

func (peer *Peer) connectionConnectedHandler() {
	// Only after the connection is established can we be sure the codec is negotiated
	codec := peer.connectionAudioInputTrack.Codec()
	audioEncoderDecoder, err := encoderdecoder.NewEncoderDecoder(codec)
	if err != nil {
		peer.logger.Error(
			"error during creation of audio encoder/decoder",
			"negotiatedCodec", codec,
			"err", err,
		)
		peer.gracefulShutdown()
		return
	}
	peer.audioEncoderDecoder = audioEncoderDecoder

	audioOutputContext, audioOutputCancelFunction := context.WithCancel(context.Background())
	peer.audioOutputChannelCancelFunc = audioOutputCancelFunction
	peer.sendAudioInputHandler()
	peer.receiveAudioOutputHandler(audioOutputContext)
}

// OnTrack handler
// Handle initialization of a new track, offered by the remote peer.
// Should start listening for packets from the remote peer, decoding them, and streaming them out
func (peer *Peer) onTrackHandler(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
	peer.logger.Debug(
		"received track",
		"track ID", tr.ID(),
		"track kind", tr.Kind().String(),
	)

	peer.connectionAudioOutputTrack = tr
}

// heartbeat onOpen handler
// Once opened, send a heartbeat message occasionally on the channel
func (peer *Peer) heartbeatOnOpenHandler() {
	heartbeatTicker := time.NewTicker(HEARTBEAT_PERIOD)
	defer heartbeatTicker.Stop()
	for {
		sendingTimestamp := <-heartbeatTicker.C
		msg, err := sendingTimestamp.MarshalBinary()
		if err != nil {
			peer.logger.Error("error while marshalling sending timestamp to binary", "err", err)
			continue
		}
		peer.logger.Debug("sending heartbeat", "sendingTimestamp", sendingTimestamp)
		if err := peer.connectionHeartbeatDataChannel.Send(msg); err != nil {
			slog.Error("error when sending heartbeat", "err", err)
		}
	}
}

// heartbeat onMessage handler
// handle a new message on the heartbeat data channel
func (peer *Peer) heartbeatOnMessageHandler(msg webrtc.DataChannelMessage) {
	currentTime := time.Now()

	var sendingTime time.Time
	sendingTime.UnmarshalBinary(msg.Data)

	networkLatency := currentTime.Sub(sendingTime)

	peer.logger.Debug(
		"received heartbeat",
		"networkLatency", networkLatency,
		"currentTime", currentTime,
		"sendingTime", sendingTime,
	)
}

// audioInputTrack onOpen handler
// Handle audio input along the audioInputChannel by forwarding through the PeerConnection audio track.
func (peer *Peer) sendAudioInputHandler() {
	go func() {
		// TODO: Race condition? Can data come in on track before connection is established and encoder/decoder is set?
		// Should we set the initial value of peer.audioEncoderDecoder to NullEncoderDecoder to handle calls to decode?

		packetTimestamp := time.Now()
		for pcmData := range peer.audioInputChannel {
			// Get the duration and update time since last sample.
			// Do this before encoding in case it takes some time,
			// or something fails.
			//
			// We need to know the time since the last sample, no matter when/if it was sent!

			duration := time.Since(packetTimestamp)
			packetTimestamp = time.Now()

			encodedData, err := peer.audioEncoderDecoder.Encode(pcmData)
			if err != nil {
				peer.logger.Error("error while encoding pcm data from input channel", "error", err)
				continue
			}

			mediaSample := media.Sample{
				Data:      encodedData,
				Duration:  duration,
				Timestamp: packetTimestamp,
			}

			peer.connectionAudioInputTrack.WriteSample(mediaSample)
		}
		// Once the audioInputChannel is closed, this go routine will die
	}()
}

// Handle audio being received by the peer and forward along audioOutputChannel.
//
// When the context is canceled, this method returns gracefully as soon as the next packet arrives.
func (peer *Peer) receiveAudioOutputHandler(ctx context.Context) {
	peer.audioOutputChannelWaitGroup.Go(func() {
		// TODO: Race condition? Can data be sent on track before connection is established and encoder/decoder is set?
		// Should we set the initial value of peer.audioEncoderDecoder to NullEncoderDecoder to handle calls to decode?
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			pkt, _, err := peer.connectionAudioOutputTrack.ReadRTP()
			if err != nil {
				peer.logger.Error("error while receiving data from remote peer", "err", err)
				continue
			}

			decodedPayload, err := peer.audioEncoderDecoder.Decode(frame.EncodedFrame(pkt.Payload))
			if err != nil {
				peer.logger.Error("error while decoding packet from remote client", "error", err)
				continue
			}
			// TODO: Handle dropped and out-of-order packets?

			select {
			case <-ctx.Done():
				return
			case peer.audioOutputChannel <- decodedPayload:

				// default:
				// If output channel cannot receive data, do we want to wait or drop the packet?
			}
		}
		// This goroutine dies when the given context is canceled, which occurs in the peer.gracefulShutdown method
	})
}

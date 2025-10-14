package peer

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hmcalister/roundtable/internal/audiodevice"
	"github.com/hmcalister/roundtable/internal/encoderdecoder"
	"github.com/hmcalister/roundtable/internal/frame"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const (
	HEARTBEAT_PERIOD time.Duration = 5 * time.Second
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

	// This context handles signalling to handlers that the peer is shutting down
	// Methods may listen for closing (calling the ctxCancelFunction), with <-ctx.Done()
	ctx           context.Context
	ctxCancelFunc context.CancelFunc

	shutdownOnce sync.Once

	// --------------------------------------------------------------------------------
	// Connection related fields

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection

	// WebRTC track for sending audio from this client to the remote client.
	// This parameter is undefined until the connection has been negotiated
	connectionAudioInputTrack *webrtc.TrackLocalStaticSample

	// WebRTC track for receiving audio from remote client.
	// This parameter is undefined until the connection has been negotiated
	connectionAudioOutputTrack *webrtc.TrackRemote

	// Data Channel to send / receive heartbeat messages on.
	// This parameter is undefined until the connection has been negotiated
	connectionHeartbeatDataChannel *webrtc.DataChannel

	// --------------------------------------------------------------------------------
	// Audio Input / Output fields

	// Audio input data from this client is passed in on this channel to be sent to remote peers.
	audioInputChannel <-chan frame.PCMFrame

	// Audio output data from this client is passed along this channel to be played on the audio output device.
	audioOutputChannel chan frame.PCMFrame

	// audioOutputChannel waitgroup, to ensure the receiveAudioOutputHandler go routine finishes
	audioOutputChannelWaitGroup sync.WaitGroup

	// Audio encoder / decoder to be used for this connection only
	audioEncoderDecoder encoderdecoder.EncoderDecoder
}

func newPeer(connection *webrtc.PeerConnection) *Peer {
	ctx, cancelFunc := context.WithCancel(context.Background())
	peer := &Peer{
		uuid:          uuid.New(),
		connection:    connection,
		ctx:           ctx,
		ctxCancelFunc: cancelFunc,
		// This a placeholder until the "real" encoder/decoder can be set
		// when the connection is established
		audioEncoderDecoder: encoderdecoder.NullEncoderDecoder{},
		audioOutputChannel:  make(chan frame.PCMFrame),
	}
	peer.logger = slog.Default().With(
		"peer uuid", peer.uuid,
	)

	peer.connection.OnConnectionStateChange(peer.onConnectionStateChangeHandler)
	peer.connection.OnTrack(peer.onTrackHandler)

	peer.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch dc.Label() {
		case "heartbeat":
			peer.setConnectionHeartbeatDataChannel(dc)
		}
	})

	return peer
}

// --------------------------------------------------------------------------------
// PUBLIC METHODS

// Get the context of this peer
// May be used to determine if the peer is shutting down by listening for <-ctx.Done()
func (peer *Peer) GetContext() context.Context {
	return peer.ctx
}

// --------------------------------------------------------------------------------
// audiodevice.AudioInputDevice Interface

// Set the audioInputChannel of this peer.
// The given channel should produce raw PCM frames from this clients audio input device (e.g. microphone)
func (peer *Peer) SetStream(c <-chan frame.PCMFrame) {
	peer.audioInputChannel = c
	peer.sendAudioInputHandler()
}

// Shutdown this peer. Handles disconnecting to remote peer and stopping streams.
// Also called the peer.ctx cancel function, so peer.ctx.Done() will signal.
//
// This function is idempotent.
func (peer *Peer) Close() {
	peer.gracefulShutdown()
}

func (peer *Peer) GetDeviceProperties() audiodevice.DeviceProperties {
	if peer.connectionAudioInputTrack == nil {
		return audiodevice.DeviceProperties{}
	}
	codec := peer.connectionAudioInputTrack.Codec()
	return audiodevice.DeviceProperties{
		SampleRate:  int(codec.ClockRate),
		NumChannels: int(codec.Channels),
	}
}

// --------------------------------------------------------------------------------
// audiodevice.AudioOutputDevice Interface

// Get the audioOutputChannel of this peer.
// The returned channel produces raw PCM Frames from the remote peer.
//
// When this peer is shutdown, the given channel is closed (hence, no data is to be sent on it anymore)
func (peer *Peer) GetStream() <-chan frame.PCMFrame {
	return peer.audioOutputChannel
}

// --------------------------------------------------------------------------------
// PRIVATE UTIL METHODS

func (peer *Peer) setConnectionHeartbeatDataChannel(dc *webrtc.DataChannel) {
	peer.connectionHeartbeatDataChannel = dc
	dc.OnOpen(peer.heartbeatOnOpenHandler)
	dc.OnMessage(peer.heartbeatOnMessageHandler)
}

func (peer *Peer) setConnectionAudioInputTrack(tr *webrtc.TrackLocalStaticSample) {
	peer.connectionAudioInputTrack = tr
}

func (peer *Peer) gracefulShutdown() {
	peer.shutdownOnce.Do(func() {
		peer.ctxCancelFunc()
		peer.connection.Close()
		peer.audioOutputChannelWaitGroup.Wait()

		if peer.audioOutputChannel != nil {
			close(peer.audioOutputChannel)
		}
	})
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

// Handle a connection being established and connected.
func (peer *Peer) connectionConnectedHandler() {
	// Only after the connection is established can we be sure the codec is negotiated
	codec := peer.connectionAudioInputTrack.Codec()
	var encoderdecoderID encoderdecoder.EncoderDecoderTypeEnum
	switch codec.MimeType {
	case webrtc.MimeTypeOpus:
		encoderdecoderID = encoderdecoder.EncoderDecoderTypeOpus
	default:
		encoderdecoderID = encoderdecoder.EncoderDecoderTypeNotImplemented
	}

	audioEncoderDecoder, err := encoderdecoder.NewEncoderDecoder(
		encoderdecoderID,
		int(codec.ClockRate),
		int(codec.Channels),
	)
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
	peer.receiveAudioOutputHandler()
}

// heartbeat onOpen handler
// Once opened, send a heartbeat message occasionally on the channel
func (peer *Peer) heartbeatOnOpenHandler() {
	heartbeatTicker := time.NewTicker(HEARTBEAT_PERIOD)
	defer heartbeatTicker.Stop()
	for {
		sendingTimestamp := <-heartbeatTicker.C
		select {
		case <-peer.ctx.Done():
			return
		default:
		}

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
		frameIndex := 0
		for {
			select {
			case <-peer.ctx.Done():
				return
			case pcmData, ok := <-peer.audioInputChannel:
				if !ok {
					return
				}
				// Get the duration and update time since last sample.
				// Do this before encoding in case it takes some time,
				// or something fails.
				//
				// We need to know the time since the last sample, no matter when/if it was sent!
				duration := time.Since(packetTimestamp)
				packetTimestamp = time.Now()

				peer.logger.Debug(
					"new frame ready",
					"frameIndex", frameIndex,
					"pcmDataLen", len(pcmData),
					"duration", duration,
				)

				encodedData, err := peer.audioEncoderDecoder.Encode(pcmData)
				if err != nil {
					peer.logger.Error(
						"error while encoding pcm data from input channel",
						"frameIndex", frameIndex,
						"pcmDataLen", len(pcmData),
						"error", err,
					)
					continue
				}

				mediaSample := media.Sample{
					Data:      encodedData,
					Duration:  duration,
					Timestamp: packetTimestamp,
				}

				peer.connectionAudioInputTrack.WriteSample(mediaSample)
				frameIndex += 1
			}
		}
		// Once the audioInputChannel is closed or the context is canceled, this go routine will die
	}()
}

// Handle audio being received by the peer and forward along audioOutputChannel.
//
// When the context is canceled, this method returns gracefully as soon as the next packet arrives.
func (peer *Peer) receiveAudioOutputHandler() {
	peer.audioOutputChannelWaitGroup.Go(func() {
		// TODO: Race condition? Can data be sent on track before connection is established and encoder/decoder is set?
		// Should we set the initial value of peer.audioEncoderDecoder to NullEncoderDecoder to handle calls to decode?

		frameIndex := 0
		for {
			select {
			case <-peer.ctx.Done():
				return
			default:
			}

			pkt, _, err := peer.connectionAudioOutputTrack.ReadRTP()
			if err != nil {
				if err == io.EOF {
					peer.logger.Debug("connection audio data track closed")
					return
				}
				peer.logger.Error(
					"error while receiving audio data from remote peer",
					"frameIndex", frameIndex,
					"err", err,
				)
				continue
			}

			decodedPayload, err := peer.audioEncoderDecoder.Decode(pkt.Payload)
			if err != nil {
				peer.logger.Error(
					"error while decoding packet from remote client",
					"frameIndex", frameIndex,
					"error", err,
				)
				continue
			}
			// TODO: Handle dropped and out-of-order packets?

			// If peer.audioOutputChannel is nil, i.e. not yet set, then this just blocks not panics
			// If output channel cannot receive data, do we want to wait or drop the packet?
			select {
			case peer.audioOutputChannel <- decodedPayload:
				// default:
			}
			slog.Debug(
				"frame sent",
				"frameIndex", frameIndex,
				"pcmDataLen", len(decodedPayload),
			)

			frameIndex += 1
		}

		// This goroutine dies when the given context is canceled, which occurs in the peer.gracefulShutdown method
	})
}

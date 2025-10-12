package peer

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
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

	uuid uuid.UUID

	// Audio input data from this client is passed along this channel.
	// This data is already OPUS encoded by the AudioManager, and converted to media.Samples
	audioInputDataChannel <-chan media.Sample

	// When finished, disconnected, or otherwise invalidated, we need to tell
	// the AudioManager we don't want more data by calling this cancel function.
	audioInputDataChannelCancelFunc context.CancelFunc

	// Audio output data from this client is passed along this channel
	// to be played on the audio output device.
	// The Peer must extract the media.Sample.Data to send along this channel.
	// When done, just close the channel.
	audioOutputDataChannel chan<- []byte

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection

	// WebRTC track for sending audio from this client to the remote client
	connectionAudioInputTrack *webrtc.TrackLocalStaticSample
}

func (peer *Peer) gracefulShutdown() {
	peer.audioInputDataChannelCancelFunc()
	close(peer.audioOutputDataChannel)
	peer.connection.Close()
}

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

// handle a new message on the heartbeat data channel
func (peer *Peer) heartbeatSendMessageHandler(dc *webrtc.DataChannel) {
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
		if err := dc.Send(msg); err != nil {
			slog.Error("error when sending heartbeat", "err", err)
		}
	}
}

// Handle audio input along the audioInputDataChannel by forwarding through the PeerConnection audio track
// This method blocks waiting for data on the audioInputDataChannel, so run in a goroutine
func (peer *Peer) sendAudioInputHandler() {
	for nextSample := range peer.audioInputDataChannel {
		// Forward the input data along the connection
		peer.connectionAudioInputTrack.WriteSample(nextSample)

	}
}

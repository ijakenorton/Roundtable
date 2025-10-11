package peer

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

const (
	HEARTBEAT_PERIOD time.Duration = 10 * time.Second
)

type Peer struct {
	logger *slog.Logger

	uuid uuid.UUID

	// Handles the connection between this client and the remote, peer client
	connection *webrtc.PeerConnection
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

package peer

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type PeerFactory struct {
	logger                       *slog.Logger
	audioTrackRTPCodecCapability webrtc.RTPCodecCapability
}

// Create a new PeerFactory.
//
// audioTrackRTPCodecCapability defines the configuration to use for all audio tracks created on peer connections.
// See https://github.com/pion/webrtc for details on these options.
//
// logger allows for a child logger to be used specifically for this client. Create a child logger like:
// ```go
// childLogger := slog.Default().With(
//
//	slog.Group("PeerFactory"),
//
// )
// ```
// If no logger is given, slog.Default() is used.
func NewPeerFactory(
	audioTrackRTPCodecCapability webrtc.RTPCodecCapability,
	logger *slog.Logger,
) *PeerFactory {
	if logger == nil {
		logger = slog.Default()
	}

	factory := &PeerFactory{
		logger:                       logger,
		audioTrackRTPCodecCapability: audioTrackRTPCodecCapability,
	}

	return factory
}

// Handle connection set up common to both offering and answering clients
func (factory *PeerFactory) commonConnectionSetup(peer *Peer) {
	peer.connection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		peer.logger.Debug("peer connection state change", "new state", pcs.String())
		switch pcs {
		case webrtc.PeerConnectionStateConnected:
			peer.logger.Info("peer connection connected")
		case webrtc.PeerConnectionStateDisconnected:
			peer.logger.Info("peer connection disconnected")
			// TODO: Handle disconnected connection
			// Should close peer.connection, somehow signal this peer is to be discarded
		case webrtc.PeerConnectionStateClosed:
			peer.logger.Info("peer connection closed")
			// TODO: Handle closed connection
			// Same as disconnected, close peer.connection, signal discard
		}
	})

	peer.connection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		peer.logger.Debug(
			"received track",
			"track ID", tr.ID(),
			"track kind", tr.Kind().String(),
		)

	})
}

// Create a new audio track to communicate between Peers.
//
// Semantically, the returned track is intended to be used to send data,
// and is listened to the receiving peer.
// That is, the caller of this function should add the returned track to
// the PeerConnection using AddTrack, and the receiving peer's connection
// should have an OnTrack handler (e.g. the one set in commonConnectionSetup).
func (factory *PeerFactory) newAudioTrack(peer *Peer) (*webrtc.TrackLocalStaticSample, error) {
	trackID := fmt.Sprintf("%s audio", peer.uuid.String())
	streamID := fmt.Sprintf("%s audio stream", peer.uuid.String())
	track, err := webrtc.NewTrackLocalStaticSample(
		factory.audioTrackRTPCodecCapability,
		trackID,
		streamID,
	)
	if err != nil {
		return nil, err
	}

	return track, nil
}

// Handle creation of a new peer on the offering side of the connection.
//
// Takes a created (but not processed) *webrtc.PeerConnection, and adds
// heartbeat and outgoing audio track.
//
// If anything goes wrong, this method returns a nil Peer and a non-nil error.
func (factory *PeerFactory) NewOfferingPeer(connection *webrtc.PeerConnection) (*Peer, error) {
	peer := &Peer{
		uuid:       uuid.New(),
		connection: connection,
	}
	peer.logger = slog.Default().With(
		"peer uuid", peer.uuid,
	)
	factory.commonConnectionSetup(peer)

	// TODO: Complete creation of datachannel before offer is made
	heartbeatDataChannel, err := connection.CreateDataChannel("heartbeat", &webrtc.DataChannelInit{})
	if err != nil {
		peer.logger.Error("error while creating heartbeat channel", "err", err)
		return nil, err
	}
	heartbeatDataChannel.OnOpen(func() { peer.heartbeatSendMessageHandler(heartbeatDataChannel) })
	heartbeatDataChannel.OnMessage(peer.heartbeatOnMessageHandler)

	track, err := factory.newAudioTrack(peer)
	if err != nil {
		factory.logger.Error("error while creating new audio track for peer", "err", err)
		return nil, err
	}
	_, err = connection.AddTrack(track)
	if err != nil {
		factory.logger.Error("error while adding audio track to peer connection", "err", err)
		return nil, err
	}

	return peer, nil
}

// Handle creation of a new peer on the answering side of the connection.
//
// Takes a created (but not processed) *webrtc.PeerConnection, and adds
// an outgoing audio track. The heartbeat channel is made by the offering peer.
//
// If anything goes wrong, this method returns a nil Peer and a non-nil error.
func (factory *PeerFactory) NewAnsweringPeer(connection *webrtc.PeerConnection) (*Peer, error) {
	peer := &Peer{
		uuid:       uuid.New(),
		connection: connection,
	}
	peer.logger = slog.Default().With(
		"peer uuid", peer.uuid,
	)
	factory.commonConnectionSetup(peer)

	peer.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch dc.Label() {
		case "heartbeat":
			dc.OnOpen(func() { peer.heartbeatSendMessageHandler(dc) })
			dc.OnMessage(peer.heartbeatOnMessageHandler)
		}
	})

	track, err := factory.newAudioTrack(peer)
	if err != nil {
		factory.logger.Error("error while creating new audio track for peer", "err", err)
		return nil, err
	}
	_, err = connection.AddTrack(track)
	if err != nil {
		factory.logger.Error("error while adding audio track to peer connection", "err", err)
		return nil, err
	}

	return peer, nil
}

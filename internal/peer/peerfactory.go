package peer

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

type PeerFactory struct {
	logger *slog.Logger

	audioTrackRTPCodecCapability webrtc.RTPCodecCapability
}

// Create a new PeerFactory.
//
// audioTrackRTPCodecCapability defines the configuration to use for all audio tracks created on peer connections.
// Effectively says what the microphone input sample rate/channels are.
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

// --------------------------------------------------------------------------------
// SETUP METHODS
// Methods to initialize important peer properties, including connection handlers

// Handle connection state changes

// Handle connection set up common to both offering and answering clients
func (factory *PeerFactory) connectionBaseSetup(peer *Peer) {
	peer.connection.OnConnectionStateChange(peer.onConnectionStateChangeHandler)
	peer.connection.OnTrack(peer.onTrackHandler)

	peer.connection.OnDataChannel(func(dc *webrtc.DataChannel) {
		switch dc.Label() {
		case "heartbeat":
			peer.setConnectionHeartbeatDataChannel(dc)
		}
	})
}

// Create an audio track and start streaming audio packets along it.streaming audio along it.
// This function:
// - Creates a new TrackLocalStaticSample, using the factory's CodecCapability
// - Attaches that sample to the peer's connection
// - Sets connectionAudioInputTrack on the Peer struct
//
// Once the connection has been fully established, the track's data should be checked
// for the negotiated codec and properties (e.g. sample rate, channels) and
// the peer's encoder/decoder should be set.
func (factory *PeerFactory) connectionAudioInputTrackSetup(peer *Peer) error {
	trackID := fmt.Sprintf("%s audio", peer.uuid.String())
	streamID := fmt.Sprintf("%s audio stream", peer.uuid.String())
	track, err := webrtc.NewTrackLocalStaticSample(
		factory.audioTrackRTPCodecCapability,
		trackID,
		streamID,
	)
	if err != nil {
		return err
	}

	_, err = peer.connection.AddTrack(track)
	if err != nil {
		return err
	}

	peer.connectionAudioInputTrack = track

	return nil
}

// --------------------------------------------------------------------------------
// PEER CREATION
// Methods to create new peers
// Split by offering peers and answering peers
// Offering peers create the meta-data channels (heartbeat, etc)

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
	factory.connectionBaseSetup(peer)

	// --------------------------------------------------------------------------------
	// Audio track setup
	err := factory.connectionAudioInputTrackSetup(peer)
	if err != nil {
		factory.logger.Error("error while creating new audio track for peer", "err", err)
		return nil, err
	}

	// --------------------------------------------------------------------------------
	// Start heartbeat data channel for network latency check between peers

	heartbeatDataChannel, err := connection.CreateDataChannel("heartbeat", &webrtc.DataChannelInit{})
	if err != nil {
		peer.logger.Error("error while creating heartbeat channel", "err", err)
		return nil, err
	}
	// Unfortunately, creating a channel does not trigger the OnDataChannel callback defined in connectionBaseSetup
	peer.setConnectionHeartbeatDataChannel(heartbeatDataChannel)

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
	factory.connectionBaseSetup(peer)
	err := factory.connectionAudioInputTrackSetup(peer)
	if err != nil {
		factory.logger.Error("error while creating new audio track for peer", "err", err)
		return nil, err
	}

	return peer, nil
}

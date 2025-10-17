package main

import (
	"context"
	"flag"
	"log/slog"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/cmd/config"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/encoderdecoder"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/networking"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/peer"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/utils"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func initializeConnectionManager(localPeerIdentifier signalling.PeerIdentifier) *networking.WebRTCConnectionManager {
	// avoid polluting the main namespace with the options and config structs

	codecs, err := utils.GetUserAuthorizedCodecs(viper.GetStringSlice("codecs"))
	if err != nil {
		slog.Error("error when loading user authorized codecs", "err", err)
		panic(err)
	}
	if len(codecs) == 0 {
		slog.Error("at least one codec must be authorized in config")
		panic("no codecs authorized")
	}
	slog.Debug("authorized codecs", "codecs", codecs)

	peerFactory := peer.NewPeerFactory(
		codecs[0],
		encoderdecoder.OPUSFrameDuration(viper.GetDuration("OPUSFrameDuration")),
		slog.Default(),
	)

	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: viper.GetStringSlice("ICEServers")}},
	}

	offerOptions := webrtc.OfferOptions{}
	answerOptions := webrtc.AnswerOptions{}

	return networking.NewWebRTCConnectionManager(
		viper.GetInt("localport"),
		viper.GetString("signallingserver"),
		peerFactory,
		localPeerIdentifier,
		codecs,
		webrtcConfig,
		offerOptions,
		answerOptions,
		slog.Default(),
	)
}

func main() {
	configFilePath := flag.String("configFilePath", "config.yaml", "Set the file path to the config file.")
	audioFile := flag.String("audioFile", "", "Set the file path to the audio file to play.")
	flag.Parse()

	config.LoadConfig(*configFilePath)
	logFilePointer, err := utils.ConfigureDefaultLogger(
		viper.GetString("loglevel"),
		viper.GetString("logfile"),
		slog.HandlerOptions{},
	)
	if err != nil {
		slog.Error("error while configuring default logger", "err", err)
		panic(err)
	}
	if logFilePointer != nil {
		defer logFilePointer.Close()
	}

	// --------------------------------------------------------------------------------
	// Set the local peer identifier to offer to peers

	localPeerIdentifier := signalling.PeerIdentifier{
		Uuid:     uuid.New(),
		PublicIP: "", // In a real client, one would need to query a STUN server to retrieve this
	}

	// --------------------------------------------------------------------------------

	inputDevice, err := device.NewFileAudioInputDevice(
		*audioFile,
		10*time.Millisecond,
	)
	if err != nil {
		slog.Error("error while opening file for audio input device", "err", err)
		return
	}

	// --------------------------------------------------------------------------------

	connectionManager := initializeConnectionManager(localPeerIdentifier)

	// --------------------------------------------------------------------------------
	// Make an offer to the answering client on 127.0.0.1:1067

	// In the real client, one would get this information as a BASE64 encoded JSON string,
	// then unmarshal into this struct. We forgo this for simplicity.
	remotePeerInformation := signalling.PeerIdentifier{
		Uuid:     uuid.UUID{},
		PublicIP: "http://127.0.0.1:1067",
	}

	ctx := context.Background()
	peer, err := connectionManager.Dial(ctx, remotePeerInformation)
	if err != nil {
		slog.Error("error during dial of answering client", "err", err)
		return
	}
	slog.Debug("established new connection", "codec", peer.GetDeviceProperties())

	// --------------------------------------------------------------------------------
	// Play some audio across the connection

	codec := peer.GetDeviceProperties()
	processedInput, _ := device.NewAudioFormatConversionDevice(
		inputDevice.GetDeviceProperties(),
		codec,
	)

	processedInput.SetStream(inputDevice.GetStream())
	peer.SetStream(processedInput.GetStream())

	inputDevice.Play(context.Background())

	// --------------------------------------------------------------------------------
	// Wait some time for pings to be exchanged
	t := time.NewTimer(10 * time.Second)
	<-t.C

	// Shut down peer and disconnect from remote
	slog.Info("Shutting down peer")
	peer.Close()
	<-peer.GetContext().Done()
	slog.Info("Shutting down peer again, for idempotency test")
	peer.Close()
}

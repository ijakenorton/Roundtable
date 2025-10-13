package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log/slog"
	"time"

	"github.com/hmcalister/roundtable/cmd/client/config"
	"github.com/hmcalister/roundtable/internal/networking"
	"github.com/hmcalister/roundtable/internal/peer"
	"github.com/hmcalister/roundtable/internal/utils"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func initializeConnectionManager() *networking.WebRTCConnectionManager {
	// avoid polluting the main namespace with the options and config structs

	audioTrackRTPCodecCapability := webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeOpus,
	}
	peerFactory := peer.NewPeerFactory(
		audioTrackRTPCodecCapability,
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
		webrtcConfig,
		offerOptions,
		answerOptions,
		slog.Default(),
	)
}

func main() {
	configFilePath := flag.String("configFilePath", "config.yaml", "Set the file path to the config file.")
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

	connectionManager := initializeConnectionManager()

	// --------------------------------------------------------------------------------
	// Make an offer to the answering client on 127.0.0.1:1067

	remoteEndpoint := base64.StdEncoding.EncodeToString([]byte("http://127.0.0.1:1067"))
	ctx := context.Background()
	peer, err := connectionManager.Dial(ctx, remoteEndpoint)
	if err != nil {
		slog.Error("error during dial of answering client", "err", err)
		return
	}

	// Wait some time for pings to be exchanged
	t := time.NewTimer(10 * time.Second)
	<-t.C

	// Shut down peer and disconnect from remote
	slog.Info("Shutting down peer")
	peer.Shutdown()
	<-peer.GetContext().Done()
	slog.Info("Shutting down peer again, for idempotency")
	peer.Shutdown()
}

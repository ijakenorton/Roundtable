package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log/slog"
	"time"

	"github.com/hmcalister/roundtable/cmd/client/config"
	"github.com/hmcalister/roundtable/internal/audiodevice/device"
	"github.com/hmcalister/roundtable/internal/networking"
	"github.com/hmcalister/roundtable/internal/peer"
	"github.com/hmcalister/roundtable/internal/utils"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func initializeConnectionManager() *networking.WebRTCConnectionManager {
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

	peerFactory := peer.NewPeerFactory(
		codecs[0],
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

	utils.SetViperDefaults()
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

	inputDevice, err := device.NewFileAudioInputDevice(
		*audioFile,
		20*time.Millisecond,
	)
	if err != nil {
		slog.Error("error while opening file for audio input device", "err", err)
		return
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

	// --------------------------------------------------------------------------------
	// Play some audio across the connection

	audioInput := inputDevice.GetStream()
	peer.SetStream(audioInput)
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

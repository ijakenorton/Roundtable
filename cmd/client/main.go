package main

import (
	"flag"
	"log/slog"

	"github.com/hmcalister/roundtable/cmd/client/config"
	"github.com/hmcalister/roundtable/internal/networking"
	"github.com/hmcalister/roundtable/internal/utils"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func initializeConnectionManager() *networking.WebRTCConnectionManager {
	// avoid polluting the main namespace with the options and config structs

	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: viper.GetStringSlice("ICEServers")}},
	}

	offerOptions := webrtc.OfferOptions{}
	answerOptions := webrtc.AnswerOptions{}

	return networking.NewWebRTCConnectionManager(
		viper.GetInt("localport"),
		viper.GetString("signallingserver"),
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

	// Keep process alive for pings to pass
	select {}
}

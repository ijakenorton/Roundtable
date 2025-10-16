package main

import (
	"flag"
	"fmt"
	"log/slog"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/cmd/client/config"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/utils"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/networking"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/peer"
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
	slog.Debug("authorized codecs", "codecs", codecs)

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

	// --------------------------------------------------------------------------------

	connectionManager := initializeConnectionManager()

	// --------------------------------------------------------------------------------

	connectionID := 0
	for {
		newPeer := <-connectionManager.IncomingConnectionChannel
		slog.Debug("received new connection", "codec", newPeer.GetDeviceProperties())
		fileName := fmt.Sprintf("connection%d.wav", connectionID)
		connectionID += 1
		go func() {
			codec := newPeer.GetDeviceProperties()
			fileProperties := audiodevice.DeviceProperties{
				SampleRate:  48000,
				NumChannels: 2,
			}
			processedOutput, _ := device.NewAudioFormatConversionDevice(
				codec,
				fileProperties,
			)
			processedOutput.SetStream(newPeer.GetStream())

			outputDevice, err := device.NewFileAudioOutputDevice(
				fileName,
				fileProperties.SampleRate,
				fileProperties.NumChannels,
			)
			if err != nil {
				slog.Error("error when creating new file audioOutputDevice", "err", err)
				newPeer.Close()
				return
			}

			outputDevice.SetStream(processedOutput.GetStream())
		}()
	}
}

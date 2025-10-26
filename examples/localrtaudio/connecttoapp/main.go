package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/cmd/application"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/cmd/config"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/audioapi"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/encoderdecoder"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/networking"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/peer"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/utils"
	// "github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/audiodevice/device"
	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/pkg/signalling"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func initializeConnectionManager(localPeerIdentifier signalling.PeerIdentifier) *networking.ConnectionManager {
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

	// --------------------------------------------------------------------------------

	opusFactory, err := encoderdecoder.NewOpusFactory(
		viper.GetDuration("OPUSFrameDuration"),
		viper.GetInt("OPUSBufferSafetyFactor"),
	)
	if err != nil {
		slog.Error("error when creating OPUS factory", "err", err)
		panic(err)
	}

	peerFactory := peer.NewPeerFactory(
		codecs[0],
		opusFactory,
		slog.Default(),
	)

	// --------------------------------------------------------------------------------

	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: viper.GetStringSlice("ICEServers")}},
	}

	offerOptions := webrtc.OfferOptions{}
	answerOptions := webrtc.AnswerOptions{}

	return networking.NewConnectionManager(
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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	signalInterruptContext, signalInterruptContextCancel := context.WithCancel(context.Background())
	go func() {
		<-sigs
		signal.Reset()
		signalInterruptContextCancel()
	}()

	localPeerIdentifier := signalling.PeerIdentifier{
		Uuid:     uuid.New(),
		PublicIP: "", // In a real client, one would need to query a STUN server to retrieve this
	}
	connectionManager := initializeConnectionManager(localPeerIdentifier)

	// Create RtAudio input device (microphone)
	frameDuration := 20 * time.Millisecond
	api, err := audioapi.NewRtAudioApi(frameDuration)
	if err != nil {
		slog.Error("error while creating rtaudio api", "err", err)
		return
	}

	app, err := application.NewApp(api, connectionManager)
	if err != nil {
		slog.Error("error while creating app", "err", err)
		return
	}

	outputDevs := api.OutputDevices()
	var ioDevice audioapi.AudioIODevice
	for _, dev := range outputDevs {
		if dev.ID == 130 {
			ioDevice = dev

		}
	}

	monitorOutputDev, err := api.InitOutputDeviceFromID(ioDevice)
	if err != nil {
		slog.Error("error while creating monitorOutputDev", "err", err)
		return
	}
	app.SetOutputDevice(monitorOutputDev)

	// Give the answering peer time to start up
	slog.Info("Waiting 2 seconds for answering peer to be ready...")
	time.Sleep(2 * time.Second)

	// --------------------------------------------------------------------------------
	// Make an offer to the answering client on 127.0.0.1:1067

	remotePeerIdentifier := signalling.PeerIdentifier{
		Uuid:     uuid.UUID{},
		PublicIP: "http://127.0.0.1:1067",
	}
	jsonPeerIdentifier, _ := json.Marshal(remotePeerIdentifier)
	encodedPeerIdentifier := base64.StdEncoding.EncodeToString(jsonPeerIdentifier)

	ctx := context.Background()
	if err := app.DialRemotePeer(ctx, encodedPeerIdentifier); err != nil {
		slog.Error("error during dial of answering client", "err", err)
		return
	}

	// --------------------------------------------------------------------------------
	// Shut down peer and disconnect from remote

	<-signalInterruptContext.Done()
	slog.Info("Shutting down app")
	app.Close()
}

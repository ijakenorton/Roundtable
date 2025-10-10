package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"time"

	"github.com/hmcalister/roundtable/cmd/client/config"
	"github.com/pion/webrtc/v4"
	"github.com/spf13/viper"
)

func main() {
	configFilePath := flag.String("configFilePath", "config.yaml", "Set the file path to the config file.")
	flag.Parse()

	config.LoadConfig(*configFilePath)
	logFilePointer := config.ConfigureLogger()
	if logFilePointer != nil {
		defer logFilePointer.Close()
	}

	// --------------------------------------------------------------------------------

	webrtcServer := webrtc.ICEServer{
		URLs: viper.GetStringSlice("ICEServers"),
	}

	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{webrtcServer},
	}

	peerOne, errOne := webrtc.NewPeerConnection(webrtcConfig)
	peerTwo, errTwo := webrtc.NewPeerConnection(webrtcConfig)
	if err := errors.Join(errOne, errTwo); err != nil {
		slog.Error("error when creating peer connection",
			"err", err,
			"webrtcConfig", webrtcConfig,
		)
		panic(err)
	}

	// --------------------------------------------------------------------------------

	peerOne.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		slog.Info(
			"peer one connection state change",
			"peer connection state", pcs.String(),
		)
	})

	peerTwo.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		slog.Info(
			"peer two connection state change",
			"peer connection state", pcs.String(),
		)
	})

	// --------------------------------------------------------------------------------

	dataChannelOptions := &webrtc.DataChannelInit{}

	dataChannel, err := peerOne.CreateDataChannel("pingChannel", dataChannelOptions)
	if err != nil {
		slog.Error("error when creating data channel",
			"err", err,
			"dataChannelOptions", dataChannelOptions,
		)
		panic(err)
	}

	dataChannel.OnOpen(func() {
		slog.Info("peer one opened data channel")
		go func() {
			for i := 0; ; i += 1 {
				msg := fmt.Sprintf("Ping %d", i)
				slog.Info("peer one sending ping", "msg", msg)
				if err := dataChannel.SendText(msg); err != nil {
					slog.Error("error when sending ping", "err", err)
				}
				time.Sleep(time.Second)
			}
		}()
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		slog.Info("peer one received", "message", string(msg.Data))
	})

	peerTwo.OnDataChannel(func(dc *webrtc.DataChannel) {
		slog.Info("peer two received data channel",
			"data channel label", dc.Label(),
			"data channel ID", dc.ID(),
		)

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			slog.Info("peer two received", "msg", string(msg.Data))
			reply := fmt.Sprintf("pong %s", string(msg.Data))
			slog.Info("peer two sending", "reply", reply)
			dc.SendText(reply)
		})
	})

	// --------------------------------------------------------------------------------

	offerOptions := &webrtc.OfferOptions{}
	answerOptions := &webrtc.AnswerOptions{}

	slog.Info("creating offer")
	offer, err := peerOne.CreateOffer(offerOptions)
	if err != nil {
		slog.Error(
			"error when creating offer",
			"err", err,
			"offerOptions", offerOptions,
		)
		panic(err)
	}

	if err = peerOne.SetLocalDescription(offer); err != nil {
		slog.Error(
			"error when setting local description of offer",
			"err", err,
			"offer", offer,
		)
		panic(err)
	}

	<-webrtc.GatheringCompletePromise(peerOne)

	if err = peerTwo.SetRemoteDescription(*peerOne.LocalDescription()); err != nil {
		slog.Error(
			"error when setting remote description of offer",
			"err", err,
		)
		panic(err)
	}

	slog.Info("creating answer")
	answer, err := peerTwo.CreateAnswer(answerOptions)
	if err != nil {
		slog.Error(
			"error when creating answer",
			"err", err,
			"answerOptions", answerOptions,
		)
		panic(err)
	}

	if err = peerTwo.SetLocalDescription(answer); err != nil {
		slog.Error(
			"error when setting local description of answer",
			"err", err,
			"answer", answer,
		)
		panic(err)
	}

	<-webrtc.GatheringCompletePromise(peerTwo)

	if err = peerOne.SetRemoteDescription(*peerTwo.LocalDescription()); err != nil {
		slog.Error(
			"error when setting remote description of answer",
			"err", err,
		)
		panic(err)
	}

	// --------------------------------------------------------------------------------

	// Keep process alive for pings to pass
	select {}
}

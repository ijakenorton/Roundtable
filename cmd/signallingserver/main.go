package main

import (
	"flag"
	"log/slog"
	"net/http"

	"github.com/hmcalister/roundtable/cmd/signallingserver/config"
	"github.com/spf13/viper"
)

func handleSignalOffer(w http.ResponseWriter, r *http.Request) {

}

func main() {
	configFilePath := flag.String("configFilePath", "config.yaml", "Set the file path to the config file.")
	flag.Parse()

	config.LoadConfig(*configFilePath)
	logFilePointer := config.ConfigureLogger()
	if logFilePointer != nil {
		defer logFilePointer.Close()
	}

	// --------------------------------------------------------------------------------

	mux := http.NewServeMux()
	mux.HandleFunc("/signal", handleSignalOffer)
	listenAddress := viper.GetString("localaddress")
	slog.Debug("starting signalling server listening", "listenAddress", listenAddress)
	http.ListenAndServe(listenAddress, mux)
}

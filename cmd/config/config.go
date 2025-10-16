package config

import (
	"log/slog"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/utils"
	"github.com/spf13/viper"
)

func LoadConfig(configFilePath string) {
	utils.SetViperDefaults()

	viper.SetConfigFile(configFilePath)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Info("no config file found", "configFilePath", configFilePath)
		} else {
			slog.Error("error during config read", "err", err)
			panic(err)
		}
	}

	// The user *must* specify at least one ICE Server
	if !viper.IsSet("ICEServers") || len(viper.GetStringSlice("ICEServers")) == 0 {
		slog.Error("at least one ICE server must be specified. See the `config` section of the README.")
		panic("no ICE server specified")
	}
}

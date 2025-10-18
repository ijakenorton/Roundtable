package config

import (
	"log/slog"

	"github.com/Honorable-Knights-of-the-Roundtable/roundtable/internal/encoderdecoder"
	"github.com/spf13/viper"
)

func setViperDefaults() {
	viper.SetDefault("loglevel", "info")
	viper.SetDefault("logfile", "")
	viper.SetDefault("localport", 1066)
	viper.SetDefault("timeout", 30)
	viper.SetDefault("codecs", []string{"CodecOpus48000Mono", "CodecOpus24000Mono", "CodecOpus48000Stereo", "CodecOpus24000Stereo"})
	viper.SetDefault("OPUSFrameDuration", encoderdecoder.OPUS_FRAME_DURATION_20_MS)
	viper.SetDefault("OPUSBufferSafetyFactor", 16)
}

func LoadConfig(configFilePath string) {
	setViperDefaults()

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

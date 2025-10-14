package utils

import "github.com/spf13/viper"

// Set the viper defaults for a Roundtable client
// For use in cmd/client, as well as several examples.
func SetViperDefaults() {
	viper.SetDefault("loglevel", "info")
	viper.SetDefault("logfile", "")
	viper.SetDefault("localport", 1066)
	viper.SetDefault("timeout", 30)
	viper.SetDefault("codecs", []string{"CodecOpus48000Stereo", "CodecOpus48000Mono"})
}

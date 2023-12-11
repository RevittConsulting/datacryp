package config

import (
	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("httpport", 8000)
	viper.SetDefault("dbfile", "mdbx.dat")
}

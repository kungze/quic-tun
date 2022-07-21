// Package options contains flags and options for initializing an server
package options

import (
	"github.com/kungze/quic-tun/pkg/app"
	"github.com/kungze/quic-tun/pkg/json"

	"github.com/kungze/quic-tun/internal/pkg/options"
	"github.com/kungze/quic-tun/pkg/log"
)

// Options runs an quic-tun server.
type Options struct {
	ServerOptions     *options.ServerOptions     `json:"server"     mapstructure:"server"`
	RestfulAPIOptions *options.RestfulAPIOptions `json:"restfulapi" mapstructure:"restfulapi"`
	LogOptions        *log.Options               `json:"log"        mapstructure:"log"`
}

// NewOptions creates a new Options object with default parameters.
func NewOptions() *Options {
	o := Options{
		ServerOptions:     options.NewServerOptions(),
		RestfulAPIOptions: options.NewRestfulAPIOptions(),
		LogOptions:        log.NewOptions(),
	}

	return &o
}

// Flags returns flags for a specific Server by section name.
func (o *Options) Flags() (fss app.NamedFlagSets) {
	o.ServerOptions.AddFlags(fss.FlagSet("server"))
	o.RestfulAPIOptions.AddFlags(fss.FlagSet("restfulapi"))
	o.LogOptions.AddFlags(fss.FlagSet("logs"))

	return fss
}

func (o *Options) String() string {
	data, _ := json.Marshal(o)

	return string(data)
}

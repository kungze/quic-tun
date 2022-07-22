// Package options contains flags and options for initializing an server
package options

import (
	"encoding/json"

	"github.com/kungze/quic-tun/pkg/app"

	"github.com/kungze/quic-tun/pkg/options"
)

// Options runs an quic-tun server.
type Options struct {
	ServerOptions     *options.ServerOptions     `json:"server"`
	RestfulAPIOptions *options.RestfulAPIOptions `json:"restfulapi"`
}

// NewOptions creates a new Options object with default parameters.
func NewOptions() *Options {
	o := Options{
		ServerOptions:     options.NewServerOptions(),
		RestfulAPIOptions: options.NewRestfulAPIOptions(),
	}

	return &o
}

// Flags returns flags for a specific Server by section name.
func (o *Options) Flags() (fss app.NamedFlagSets) {
	o.ServerOptions.AddFlags(fss.FlagSet("server"))
	o.RestfulAPIOptions.AddFlags(fss.FlagSet("restfulapi"))

	return fss
}

func (o *Options) String() string {
	data, _ := json.Marshal(o)

	return string(data)
}

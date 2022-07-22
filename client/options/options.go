// Package options contains flags and options for initializing a server
package options

import (
	"encoding/json"

	"github.com/kungze/quic-tun/pkg/app"
	"github.com/kungze/quic-tun/pkg/options"
)

// Options runs a quic-tun client.
type Options struct {
	ClientOptions     *options.ClientOptions     `json:"client"     mapstructure:"client"`
	RestfulAPIOptions *options.RestfulAPIOptions `json:"restfulapi" mapstructure:"restfulapi"`
}

// NewOptions creates a new Options object with default parameters.
func NewOptions() *Options {
	o := Options{
		ClientOptions:     options.NewClientOptions(),
		RestfulAPIOptions: options.NewRestfulAPIOptions(),
	}

	return &o
}

// Flags returns flags for a specific Server by section name.
func (o *Options) Flags() (fss app.NamedFlagSets) {
	o.ClientOptions.AddFlags(fss.FlagSet("client"))
	o.RestfulAPIOptions.AddFlags(fss.FlagSet("restfulapi"))

	return fss
}

func (o *Options) String() string {
	data, _ := json.Marshal(o)

	return string(data)
}

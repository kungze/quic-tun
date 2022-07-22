package config

import "github.com/kungze/quic-tun/server/options"

// Config is the running configuration structure of the quictun server service.
type Config struct {
	*options.Options
}

// CreateConfigFromOptions creates a running configuration instance based
// on a given quic-tun command line or configuration file option.
func CreateConfigFromOptions(opts *options.Options) (*Config, error) {
	return &Config{opts}, nil
}

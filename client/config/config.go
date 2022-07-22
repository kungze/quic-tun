package config

import "github.com/kungze/quic-tun/client/options"

// Config is the running configuration structure of the quictun client service.
type Config struct {
	*options.Options
}

// CreateConfigFromOptions creates a running configuration instance based
// on a given quic-tun command line or configuration file option.
func CreateConfigFromOptions(opts *options.Options) (*Config, error) {
	return &Config{opts}, nil
}

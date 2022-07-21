package options

import (
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/pflag"
)

// ServerOptions contains the options while running a quic-tun server.
type ServerOptions struct {
	BindAddress       string `json:"bind-address"       mapstructure:"bind-address"`
	BindPort          int    `json:"bind-port"          mapstructure:"bind-port"`
	KeyFile           string `json:"key-file"           mapstructure:"key-file"`
	CertFile          string `json:"cert-file"          mapstructure:"cert-file"`
	CaFile            string `json:"ca-file"            mapstructure:"ca-file"`
	VerifyClient      bool   `json:"verify-client"      mapstructure:"verify-client"`
	TokenParserPlugin string `json:"token-parse-plugin" mapstructure:"token-parse-plugin"`
	TokenParserKey    string `json:"token-parse-key"    mapstructure:"token-parse-key"`
}

// Validate checks validation of ServerOptions.
func (s *ServerOptions) Validate() []error {
	if s == nil {
		return nil
	}

	errors := []error{}

	if s.BindPort < 1 || s.BindPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--server.bind-port %v must be between 1 and 65535, inclusive. It cannot be turned off with 0",
				s.BindPort,
			),
		)
	}

	return errors
}

func (s *ServerOptions) Address() string {
	return net.JoinHostPort(s.BindAddress, strconv.Itoa(s.BindPort))
}

// NewServerOptions creates a new ServerOptions object with default parameters.
func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		BindAddress:       "0.0.0.0",
		BindPort:          7500,
		VerifyClient:      false,
		TokenParserPlugin: "Cleartext",
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindAddress, "server.bind-address", s.BindAddress,
		"The ip that the server side endpoint listen on.")
	fs.IntVar(&s.BindPort, "server.bind-port", s.BindPort,
		"The port on which to server quic-tun side endpoint.")
	fs.StringVar(&s.KeyFile, "server.key-file", s.KeyFile,
		"File containing the default x509 private key matching --server.cert-file.")
	fs.StringVar(&s.CertFile, "server.cert-file", s.CertFile,
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
			"after server cert).")
	fs.StringVar(&s.CaFile, "server.ca-file", s.CaFile,
		"The certificate authority file path, used to verify client endpoint certificate. "+
			"If not specified, quictun try to load system certificate.")
	fs.BoolVar(&s.VerifyClient, "server.verify-client", s.VerifyClient,
		"Whether to require client certificate and verify it")
	fs.StringVar(&s.TokenParserPlugin, "server.token-parse-plugin", s.TokenParserPlugin,
		"The token parser plugin.")
	fs.StringVar(&s.TokenParserKey, "server.token-parse-key", s.TokenParserKey,
		"An argument to be passed to the token parse plugin on instantiation.")
}

package options

import (
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/pflag"
)

// ClientOptions contains the options while running a quic-tun client.
type ClientOptions struct {
	BindProtocol         string `json:"bind-protocol"`
	BindAddress          string `json:"bind-address"`
	BindPort             int    `json:"bind-port"`
	ServerEndpointSocket string `json:"server-endpoint"`
	TokenPlugin          string `json:"token-source-plugin"`
	TokenSource          string `json:"token-source"`
	CertFile             string `json:"cert-file"`
	KeyFile              string `json:"key-file"`
	VerifyServer         bool   `json:"verify-server"`
	CaFile               string `json:"ca-file"`
}

// Validate checks validation of ClientOptions.
func (s *ClientOptions) Validate() []error {
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

func (s *ClientOptions) Address() string {
	return net.JoinHostPort(s.BindAddress, strconv.Itoa(s.BindPort))
}

// NewServerRunOptions creates a new ClientOptions object with default parameters.
func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		BindProtocol: "tcp",
		BindAddress:  "127.0.0.1",
		BindPort:     6500,
		TokenPlugin:  "Fixed",
		VerifyServer: false,
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *ClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.BindProtocol, "client.bind-protocol", s.BindProtocol,
		"The protocol that the client side endpoint listen on. eg: tcp")
	fs.StringVar(&s.BindAddress, "client.bind-address", s.BindAddress,
		"The ip that the client side endpoint listen on.")
	fs.IntVar(&s.BindPort, "client.bind-port", s.BindPort,
		"The port on which to client quic-tun side endpoint.")
	fs.StringVar(&s.ServerEndpointSocket, "client.server-endpoint", s.ServerEndpointSocket,
		"The server side endpoint address, example: example.com:6565")
	fs.StringVar(&s.TokenPlugin, "client.token-source-plugin", s.TokenPlugin,
		"Specify the token plugin. Token used to tell the server endpoint which server app we want to access. Support values: Fixed, File.")
	fs.StringVar(&s.TokenSource, "client.token-source", s.TokenSource,
		"An argument to be passed to the token source plugin on instantiation.")
	fs.StringVar(&s.KeyFile, "client.key-file", s.KeyFile,
		"File containing the default x509 private key matching --server.cert-file.")
	fs.StringVar(&s.CertFile, "client.cert-file", s.CertFile,
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
			"after server cert).")
	fs.BoolVar(&s.VerifyServer, "client.verify-server", s.VerifyServer,
		"Whether skip verify server endpoint.")
	fs.StringVar(&s.CaFile, "client.ca-file", s.CaFile,
		"The certificate authority file path, used to verify client endpoint certificate. "+
			"If not specified, quictun try to load system certificate.")
}

package options

import (
	"fmt"
	"net"
	"strconv"

	"github.com/spf13/pflag"
)

// RestfulAPIOptions contains the options while running a API server.
type RestfulAPIOptions struct {
	APIListenAddress string `json:"api-listen-address"`
	APIListenPort    int    `json:"api-listen-port"`
}

// Validate checks validation of RestfulAPIOptions.
func (r *RestfulAPIOptions) Validate() []error {
	if r == nil {
		return nil
	}

	errors := []error{}

	if r.APIListenPort < 1 || r.APIListenPort > 65535 {
		errors = append(
			errors,
			fmt.Errorf(
				"--restfulapi.api-listen-port: %v must be between 1 and 65535, inclusive. It cannot be turned off with 0",
				r.APIListenPort,
			),
		)
	}

	return errors
}

func (s *RestfulAPIOptions) Address() string {
	return net.JoinHostPort(s.APIListenAddress, strconv.Itoa(s.APIListenPort))
}

func NewRestfulAPIOptions() *RestfulAPIOptions {
	return &RestfulAPIOptions{
		APIListenAddress: "0.0.0.0",
		APIListenPort:    8086,
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *RestfulAPIOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.APIListenAddress, "restfulapi.api-listen-address", s.APIListenAddress,
		"The address of the API(httpd) server listen on.")
	fs.IntVar(&s.APIListenPort, "restfulapi.api-listen-port", s.APIListenPort,
		"The port on which to server the API(httpd).")
}

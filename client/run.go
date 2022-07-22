package client

import "github.com/kungze/quic-tun/client/config"

// Run runs the specified Server. This should never exit.
func Run(cfg *config.Config) error {
	server, err := createClientEndpoint(cfg)
	if err != nil {
		return err
	}

	return server.PrepareRun().Run()
}

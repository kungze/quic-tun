package server

import "github.com/kungze/quic-tun/server/config"

// Run runs the specified Server. This should never exit.
func Run(cfg *config.Config) error {
	server, err := createServerEndpoint(cfg)
	if err != nil {
		return err
	}

	return server.PrepareRun().Run()
}

package client

import (
	"github.com/kungze/quic-tun/internal/client/config"
	"github.com/kungze/quic-tun/internal/client/options"
	"github.com/kungze/quic-tun/pkg/app"
	"github.com/kungze/quic-tun/pkg/log"
)

const commandDesc = `Establish a fast&security tunnel,
 make you can access remote TCP/UNIX application like local application.

Find more quic-tun information at:
    https://github.com/kungze/quic-tun/blob/master/README.md`

// NewApp creates an App object with default parameters.
func NewApp(basename string) *app.App {
	opts := options.NewOptions()
	application := app.NewApp("Start up client side endpoint",
		basename,
		app.WithOptions(opts),
		app.WithDescription(commandDesc),
		app.WithDefaultValidArgs(),
		app.WithRunFunc(run(opts)),
	)

	return application
}

func run(opts *options.Options) app.RunFunc {
	return func(basename string) error {
		log.Init(opts.LogOptions)
		defer log.Flush()

		cfg, err := config.CreateConfigFromOptions(opts)
		if err != nil {
			return err
		}

		return Run(cfg)
	}
}

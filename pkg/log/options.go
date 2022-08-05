package log

import (
	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
)

// Options contains configuration items related to log.
type Options struct {
	OutputPaths       []string `json:"log-output-paths"       mapstructure:"log-output-paths"`
	ErrorOutputPaths  []string `json:"log-error-output-paths" mapstructure:"log-error-output-paths"`
	Level             string   `json:"log-level"              mapstructure:"log-level"`
	Format            string   `json:"log-format"             mapstructure:"log-format"`
	DisableCaller     bool     `json:"log-disable-caller"     mapstructure:"log-disable-caller"`
	DisableStacktrace bool     `json:"log-disable-stacktrace" mapstructure:"log-disable-stacktrace"`
	Development       bool     `json:"log-development"        mapstructure:"log-development"`
	Name              string   `json:"log-name"               mapstructure:"log-name"`
}

// NewOptions creates an Options object with default parameters.
func NewOptions() *Options {
	return &Options{
		Level:             zapcore.InfoLevel.String(),
		DisableCaller:     false,
		DisableStacktrace: false,
		Format:            "console",
		Development:       false,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stderr"},
	}
}

// AddFlags adds flags for log to the specified FlagSet object.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Level, "log-level", o.Level, "Minimum log output `LEVEL`.")
	fs.BoolVar(&o.DisableCaller, "log-disable-caller", o.DisableCaller, "Disable output of caller information in the log.")
	fs.BoolVar(&o.DisableStacktrace, "log-disable-stacktrace",
		o.DisableStacktrace, "Disable the log to record a stack trace for all messages at or above panic level.")
	fs.StringVar(&o.Format, "log-format", o.Format, "Log output `FORMAT`, support 'console' or 'json' format.")
	fs.StringSliceVar(&o.OutputPaths, "log-output-paths", o.OutputPaths, "Output paths of log.")
	fs.StringSliceVar(&o.ErrorOutputPaths, "log-error-output-paths", o.ErrorOutputPaths, "Error output paths of log.")
	fs.BoolVar(
		&o.Development,
		"log-development",
		o.Development,
		"Development puts the logger in development mode, which changes "+
			"the behavior of DPanicLevel and takes stacktraces more liberally.",
	)
	fs.StringVar(&o.Name, "log-name", o.Name, "The name of the logger.")
}

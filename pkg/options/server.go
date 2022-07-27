package options

import "github.com/spf13/pflag"

//ServerOptions contains information for a client service.
type ServerOptions struct {
	ListenOn          string `json:"listen-on"           mapstructure:"listen-on"`
	TokenParserPlugin string `json:"token-parser-plugin" mapstructure:"token-parser-plugin"`
	TokenParserKey    string `json:"token-parser-key"    mapstructure:"token-parser-key"`
}

// GetDefaultServerOptions returns a server configuration with default values.
func GetDefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		ListenOn:          "0.0.0.0:7500",
		TokenParserPlugin: "Cleartext",
		TokenParserKey:    "",
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.ListenOn, "listen-on", s.ListenOn,
		"The socket that the server side endpoint listen on")
	fs.StringVar(&s.TokenParserPlugin, "token-parser-plugin", s.TokenParserPlugin,
		"The token parser plugin.")
	fs.StringVar(&s.TokenParserKey, "token-parser-key", s.TokenParserKey,
		"An argument to be passed to the token parse plugin on instantiation.")
}

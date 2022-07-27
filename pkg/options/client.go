package options

import "github.com/spf13/pflag"

//ClientOptions contains information for a client service.
type ClientOptions struct {
	ListenOn             string `json:"listen-on"           mapstructure:"listen-on"`
	ServerEndpointSocket string `json:"server-endpoint"     mapstructure:"server-endpoint"`
	TokenPlugin          string `json:"token-source-plugin" mapstructure:"token-source-plugin"`
	TokenSource          string `json:"token-source"        mapstructure:"token-source"`
}

// GetDefaultClientOptions returns a client configuration with default values.
func GetDefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		ListenOn:             "tcp:127.0.0.1:6500",
		ServerEndpointSocket: "",
		TokenPlugin:          "Fixed",
		TokenSource:          "",
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *ClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.ListenOn, "listen-on", s.ListenOn,
		"The socket that the client side endpoint listen on")
	fs.StringVar(&s.ServerEndpointSocket, "server-endpoint", s.ServerEndpointSocket,
		"The server side endpoint address, example: example.com:6565")
	fs.StringVar(&s.TokenPlugin, "token-source-plugin", s.TokenPlugin,
		"Specify the token plugin. Token used to tell the server endpoint which server app we want to access. Support values: Fixed, File.")
	fs.StringVar(&s.TokenSource, "token-source", s.TokenSource,
		"An argument to be passed to the token source plugin on instantiation.")
}

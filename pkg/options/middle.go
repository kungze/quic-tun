package options

import "github.com/spf13/pflag"

//MiddleOptions contains information for a middle service.
type MiddleOptions struct {
	ListenOn    string `json:"listen-on"           mapstructure:"listen-on"`
	BindUdpPort int    `json:"bind-udp-port"       mapstructure:"bind-udp-port"`
}

// GetDefaultMiddleOptions returns a middle configuration with default values.
func GetDefaultMiddleOptions() *MiddleOptions {
	return &MiddleOptions{
		ListenOn:    "0.0.0.0:3737",
		BindUdpPort: 7373,
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (m *MiddleOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&m.ListenOn, "listen-on", m.ListenOn,
		"The socket that the middle side endpoint listen on")
	fs.IntVar(&m.BindUdpPort, "bind-udp-port", m.BindUdpPort,
		"Bound udp port, currently used for hole punching")
}

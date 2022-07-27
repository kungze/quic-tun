package options

import "github.com/spf13/pflag"

// RestfulAPIOptions contains the options while running a API server.
type RestfulAPIOptions struct {
	HttpdListenOn string `json:"httpd-listen-on" mapstructure:"httpd-listen-on"`
}

func GetDefaultRestfulAPIOptions() *RestfulAPIOptions {
	return &RestfulAPIOptions{
		HttpdListenOn: "0.0.0.0:8086",
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (r *RestfulAPIOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.HttpdListenOn, "httpd-listen-on", r.HttpdListenOn,
		"The socket of the API(httpd) server listen on")
}

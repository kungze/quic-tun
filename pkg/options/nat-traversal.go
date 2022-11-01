package options

import "github.com/spf13/pflag"

type NATTraversalOptions struct {
	NATTraversalMode     bool   `json:"nat-traversal-mode"      mapstructure:"nat-traversal-mode"`
	MQTTServerURL        string `json:"mqtt-server-url"         mapstructure:"mqtt-server-url"`
	MQTTTopicKey         string `json:"mqtt-topic-key"          mapstructure:"mqtt-topic-key"`
	MQTTServerUsername   string `json:"mqtt-server-username"    mapstructure:"mqtt-server-username"`
	MQTTServerPassword   string `json:"mqtt-server-password"    mapstructure:"mqtt-server-password"`
	STUNServerURL        string `json:"stun-server-url"         mapstructure:"stun-server-url"`
	STUNServerURLConvert string `json:"stun-server-url-convert" mapstructure:"stun-server-url-convert"`
	STUNServerUsername   string `json:"stun-server-username"    mapstructure:"stun-server-username"`
	STUNServerPassword   string `json:"stun-server-password"    mapstructure:"stun-server-password"`
	STUNServerSecure     bool   `json:"stun-server-secure"      mapstructure:"stun-server-secure"`
	TURNServerURL        string `json:"turn-server-url"         mapstructure:"turn-server-url"`
	TURNServerUsername   string `json:"turn-server-username"    mapstructure:"turn-server-username"`
	TURNServerPassword   string `json:"turn-server-password"    mapstructure:"turn-server-password"`
	TURNServerSecure     bool   `json:"turn-server-secure"      mapstructure:"turn-server-secure"`
	ICEFailedTimeout     int    `json:"ice-failed-timeout"      mapstructure:"ice-failed-timeout"`
}

func GetDefaultNATTraversalOptions() *NATTraversalOptions {
	return &NATTraversalOptions{
		NATTraversalMode:     false,
		MQTTServerURL:        "broker.emqx.io:1883",
		MQTTTopicKey:         "",
		MQTTServerUsername:   "emqx",
		MQTTServerPassword:   "public",
		STUNServerURL:        "stun.voip.aebc.com",
		STUNServerURLConvert: "",
		STUNServerUsername:   "",
		STUNServerPassword:   "",
		STUNServerSecure:     false,
		TURNServerURL:        "",
		TURNServerUsername:   "",
		TURNServerPassword:   "",
		TURNServerSecure:     false,
		ICEFailedTimeout:     15,
	}
}

func (nt *NATTraversalOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&nt.NATTraversalMode, "nat-traversal-mode", nt.NATTraversalMode, "Wheter enable nat traversal mode")
	fs.StringVar(&nt.MQTTServerURL, "mqtt-server-url", nt.MQTTServerURL, "Quic-tun use mqtt server as ICE's SIP server.")
	fs.StringVar(&nt.MQTTTopicKey, "mqtt-topic-key", nt.MQTTTopicKey, "Used to identify a quictun-server")
	fs.StringVar(&nt.MQTTServerUsername, "mqtt-server-username", nt.MQTTServerUsername, "")
	fs.StringVar(&nt.MQTTServerPassword, "mqtt-server-password", nt.MQTTServerPassword, "")
	fs.StringVar(&nt.STUNServerURL, "stun-server-url", nt.STUNServerURL, "A STUN (rfc7064) URL")
	fs.StringVar(&nt.STUNServerURLConvert, "stun-server-url-convert", nt.STUNServerURLConvert, "A STUN (rfc7064) URL")
	fs.StringVar(&nt.STUNServerUsername, "stun-server-username", nt.STUNServerUsername, "The username of STUN server")
	fs.StringVar(&nt.STUNServerPassword, "stun-server-password", nt.STUNServerPassword, "The password of STUN server")
	fs.BoolVar(&nt.STUNServerSecure, "stun-server-secure", nt.STUNServerSecure, "Whether it is a STUNS (secure) server")
	fs.StringVar(&nt.TURNServerURL, "turn-server-url", nt.TURNServerURL, "A TURN (rfc7065) URL")
	fs.StringVar(&nt.TURNServerUsername, "turn-server-username", nt.TURNServerUsername, "The username of TURN server")
	fs.StringVar(&nt.TURNServerPassword, "turn-server-password", nt.TURNServerPassword, "The password of TURN server")
	fs.BoolVar(&nt.TURNServerSecure, "turn-server-secure", nt.TURNServerSecure, "Whether it is a TURNS (secure) server")
	fs.IntVar(&nt.ICEFailedTimeout, "ice-failed-timeout", nt.ICEFailedTimeout, "ICE failed timeout time, The timeout time is in seconds")
}

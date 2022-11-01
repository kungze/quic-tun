package options

import "github.com/spf13/pflag"

type NATTraversalOptions struct {
	NATTraversalMode   bool   `json:"nat-traversal-mode"   mapstructure:"nat-traversal-mode"`
	MQTTServerURL      string `json:"mqtt-server-url"      mapstructure:"mqtt-server-url"`
	MQTTTopicKey       string `json:"mqtt-topic-key"       mapstructure:"mqtt-topic-key"`
	MQTTServerUsername string `json:"mqtt-server-username" mapstructure:"mqtt-server-username"`
	MQTTServerPassword string `json:"mqtt-server-password" mapstructure:"mqtt-server-password"`
	ICEServerURL       string `json:"ice-server-url"       mapstructure:"ice-server-url"`
	ICEServerScheme    string `json:"ice-server-scheme"    mapstructure:"ice-server-scheme"`
	ICEServerUsername  string `json:"ice-server-username"  mapstructure:"ice-server-username"`
	ICEServerPassword  string `json:"ice-server-password"  mapstructure:"ice-server-password"`
}

func GetDefaultNATTraversalOptions() *NATTraversalOptions {
	return &NATTraversalOptions{
		NATTraversalMode:   false,
		MQTTServerURL:      "broker.emqx.io:1883",
		MQTTTopicKey:       "",
		MQTTServerUsername: "emqx",
		MQTTServerPassword: "public",
		ICEServerURL:       "stun.voip.blackberry.com",
		ICEServerScheme:    "stun",
		ICEServerUsername:  "",
		ICEServerPassword:  "",
	}
}

func (nt *NATTraversalOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&nt.NATTraversalMode, "nat-traversal-mode", nt.NATTraversalMode, "Wheter enable nat traversal mode")
	fs.StringVar(&nt.MQTTServerURL, "mqtt-server-url", nt.MQTTServerURL, "Quic-tun use mqtt server as ICE's SIP server.")
	fs.StringVar(&nt.ICEServerScheme, "ice-server-scheme", nt.ICEServerScheme, "STUN(S) or TURN(S)")
	fs.StringVar(&nt.MQTTTopicKey, "mqtt-topic-key", nt.MQTTTopicKey, "Used to identify a quictun-server")
	fs.StringVar(&nt.MQTTServerUsername, "mqtt-server-username", nt.MQTTServerUsername, "")
	fs.StringVar(&nt.MQTTServerPassword, "mqtt-server-password", nt.MQTTServerPassword, "")
	fs.StringVar(&nt.ICEServerURL, "ice-server-url", nt.ICEServerURL, "A STUN (rfc7064) or TURN (rfc7065) URL")
	fs.StringVar(&nt.ICEServerUsername, "ice-server-username", nt.ICEServerUsername, "The username of ICE server")
	fs.StringVar(&nt.ICEServerPassword, "ice-server-password", nt.ICEServerPassword, "The password of ICE server")
}

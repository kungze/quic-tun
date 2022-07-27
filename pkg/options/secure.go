package options

import "github.com/spf13/pflag"

// SecureOptions contains information for TLS.
type SecureOptions struct {
	KeyFile              string `json:"key-file"               mapstructure:"key-file"`
	CertFile             string `json:"cert-file"              mapstructure:"cert-file"`
	CaFile               string `json:"ca-file"                mapstructure:"ca-file"`
	VerifyRemoteEndpoint bool   `json:"verify-remote-endpoint" mapstructure:"verify-remote-endpoint"`
}

// GetDefaultSecureOptions returns a Secure configuration with default values.
func GetDefaultSecureOptions() *SecureOptions {
	return &SecureOptions{
		KeyFile:              "",
		CertFile:             "",
		CaFile:               "",
		VerifyRemoteEndpoint: false,
	}
}

// AddFlags adds flags for a specific Server to the specified FlagSet.
func (s *SecureOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.KeyFile, "key-file", s.KeyFile,
		"File containing the default x509 private key matching --cert-file.")
	fs.StringVar(&s.CertFile, "cert-file", s.CertFile,
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
			"after server cert).")
	fs.StringVar(&s.CaFile, "ca-file", s.CaFile,
		"The certificate authority file path, used to verify client endpoint certificate. "+
			"If not specified, quictun try to load system certificate.")
	fs.BoolVar(&s.VerifyRemoteEndpoint, "verify-remote-endpoint", s.VerifyRemoteEndpoint,
		"Whether to require remote endpoint certificate and verify it")
}

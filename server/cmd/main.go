/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/kungze/quic-tun/pkg/options"
	"github.com/kungze/quic-tun/pkg/restfulapi"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

var (
	serOptions *options.ServerOptions
	apiOptions *options.RestfulAPIOptions
	secOptions *options.SecureOptions
)

func buildCommand(basename string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   basename,
		Short: "Start up the server side endpoint",
		Long: `Establish a fast&security tunnel,
make you can access remote TCP/UNIX application like local application.
	   
Find more quic-tun information at:
	https://github.com/kungze/quic-tun/blob/master/README.md`,
		RunE: runCommand,
	}
	// Initialize the flags needed to start the server
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	serOptions.AddFlags(rootCmd.Flags())
	apiOptions.AddFlags(rootCmd.Flags())
	secOptions.AddFlags(rootCmd.Flags())
	options.AddConfigFlag(basename, rootCmd.Flags())

	return rootCmd
}

func runCommand(cmd *cobra.Command, args []string) error {
	options.PrintWorkingDir()
	options.PrintFlags(cmd.Flags())
	options.PrintConfig()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	if err := viper.Unmarshal(serOptions); err != nil {
		return err
	}

	if err := viper.Unmarshal(apiOptions); err != nil {
		return err
	}

	if err := viper.Unmarshal(secOptions); err != nil {
		return err
	}

	// run server
	runFunc(serOptions, apiOptions, secOptions)
	return nil
}

func runFunc(so *options.ServerOptions, ao *options.RestfulAPIOptions, seco *options.SecureOptions) {

	keyFile := seco.KeyFile
	certFile := seco.CertFile
	verifyClient := seco.VerifyRemoteEndpoint
	caFile := seco.CaFile
	tokenParserPlugin := so.TokenParserPlugin
	tokenParserKey := so.TokenParserKey

	var tlsConfig *tls.Config
	if keyFile == "" || certFile == "" {
		tlsConfig = generateTLSConfig()
	} else {
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			klog.ErrorS(err, "Certificate file or private key file is invalid.")
			return
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"quic-tun"},
		}
	}
	if verifyClient {
		if caFile == "" {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				klog.ErrorS(err, "Failed to load system cert pool")
				return
			}
			tlsConfig.ClientCAs = certPool
		} else {
			caPemBlock, err := os.ReadFile(caFile)
			if err != nil {
				klog.ErrorS(err, "Failed to read ca file.")
			}
			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(caPemBlock)
			tlsConfig.ClientCAs = certPool
		}
	}

	// Start API server
	httpd := restfulapi.NewHttpd(ao.HttpdListenOn)
	go httpd.Start()

	// Start server endpoint
	s := &server.ServerEndpoint{
		Address:     so.ListenOn,
		TlsConfig:   tlsConfig,
		TokenParser: loadTokenParserPlugin(tokenParserPlugin, tokenParserKey),
	}
	s.Start()
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-tun"},
	}
}

func loadTokenParserPlugin(plugin string, key string) token.TokenParserPlugin {
	switch strings.ToLower(plugin) {
	case "cleartext":
		return token.NewCleartextTokenParserPlugin(key)
	default:
		panic(fmt.Sprintf("Token parser plugin %s don't support", plugin))
	}
}

func main() {
	// Initialize the options needed to start the server
	serOptions = options.GetDefaultServerOptions()
	apiOptions = options.GetDefaultRestfulAPIOptions()
	secOptions = options.GetDefaultSecureOptions()
	rootCmd := buildCommand("quictun-server")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

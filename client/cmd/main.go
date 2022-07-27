/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kungze/quic-tun/client"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/kungze/quic-tun/pkg/restfulapi"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

var (
	clientOptions *options.ClientOptions
	apiOptions    *options.RestfulAPIOptions
	secOptions    *options.SecureOptions
)

func buildCommand(basename string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   basename,
		Short: "Start up the client side endpoint",
		Long: `Establish a fast&security tunnel,
make you can access remote TCP/UNIX application like local application.
	   
Find more quic-tun information at:
	https://github.com/kungze/quic-tun/blob/master/README.md`,
		RunE: runCommand,
	}
	// Initialize the flags needed to start the server
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	clientOptions.AddFlags(rootCmd.Flags())
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

	if err := viper.Unmarshal(clientOptions); err != nil {
		return err
	}

	if err := viper.Unmarshal(apiOptions); err != nil {
		return err
	}

	if err := viper.Unmarshal(secOptions); err != nil {
		return err
	}

	// run server
	runFunc(clientOptions, apiOptions, secOptions)
	return nil
}

func runFunc(co *options.ClientOptions, ao *options.RestfulAPIOptions, seco *options.SecureOptions) {
	localSocket := co.ListenOn
	serverEndpointSocket := co.ServerEndpointSocket
	tokenPlugin := co.TokenPlugin
	tokenSource := co.TokenSource
	certFile := seco.CertFile
	keyFile := seco.KeyFile
	caFile := seco.CaFile
	verifyServer := seco.VerifyRemoteEndpoint
	apiListenOn := ao.HttpdListenOn

	tlsConfig := &tls.Config{
		InsecureSkipVerify: !verifyServer,
		NextProtos:         []string{"quic-tun"},
	}
	if certFile != "" && keyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			klog.ErrorS(err, "Certificate file or private key file is invalid.")
			return
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}
	if caFile != "" {
		caPemBlock, err := os.ReadFile(caFile)
		if err != nil {
			klog.ErrorS(err, "Failed to read ca file.")
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caPemBlock)
		tlsConfig.RootCAs = certPool
	} else {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			klog.ErrorS(err, "Failed to load system cert pool")
			return
		}
		tlsConfig.ClientCAs = certPool
	}

	// Start API server
	httpd := restfulapi.NewHttpd(apiListenOn)
	go httpd.Start()

	// Start client endpoint
	c := client.ClientEndpoint{
		LocalSocket:          localSocket,
		ServerEndpointSocket: serverEndpointSocket,
		TokenSource:          loadTokenSourcePlugin(tokenPlugin, tokenSource),
		TlsConfig:            tlsConfig,
	}
	c.Start()
}

func loadTokenSourcePlugin(plugin string, source string) token.TokenSourcePlugin {
	switch strings.ToLower(plugin) {
	case "fixed":
		return token.NewFixedTokenPlugin(source)
	case "file":
		return token.NewFileTokenSourcePlugin(source)
	default:
		panic(fmt.Sprintf("The token source plugin %s is invalid", plugin))
	}
}

func main() {
	// Initialize the options needed to start the server
	clientOptions = options.GetDefaultClientOptions()
	apiOptions = options.GetDefaultRestfulAPIOptions()
	secOptions = options.GetDefaultSecureOptions()
	rootCmd := buildCommand("quictun-client")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

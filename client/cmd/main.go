/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/kungze/quic-tun/client"
	"github.com/kungze/quic-tun/pkg/restfulapi"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	localSocket          string
	serverEndpointSocket string
	tokenPlugin          string
	tokenSource          string
	certFile             string
	keyFile              string
	caFile               string
	insecureSkipVerify   bool
	apiListenOn          string
)

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
	rootCmd := &cobra.Command{
		Use:   "quictun-client",
		Short: "Start up the client side endpoint",
		Run: func(cmd *cobra.Command, args []string) {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
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
			httpd, ch := restfulapi.NewHttpd(apiListenOn)
			go httpd.Start()

			// Start client endpoint
			c := client.ClientEndpoint{
				LocalSocket:          localSocket,
				ServerEndpointSocket: serverEndpointSocket,
				TokenSource:          loadTokenSourcePlugin(tokenPlugin, tokenSource),
				TlsConfig:            tlsConfig,
				TunCh:                ch,
			}
			c.Start()
		},
	}
	defer klog.Flush()
	rootCmd.PersistentFlags().StringVar(&localSocket, "listen-on", "tcp:127.0.0.1:6500", "The socket that the client side endpoint listen on.")
	rootCmd.PersistentFlags().StringVar(&serverEndpointSocket, "server-endpoint", "", "The server side endpoint address, example: example.com:6565")
	rootCmd.PersistentFlags().StringVar(
		&tokenPlugin, "token-source-plugin", "Fixed",
		"Specify the token plugin. Token used to tell the server endpoint which server app we want to access. Support values: Fixed, File.")
	rootCmd.PersistentFlags().StringVar(&tokenSource, "token-source", "", "An argument to be passed to the token source plugin on instantiation.")
	rootCmd.PersistentFlags().StringVar(&certFile, "cert-file", "", "The certificate file path, this is required if the --verify-client is True in server endpoint.")
	rootCmd.PersistentFlags().StringVar(&keyFile, "key-file", "", "The private key file path, this is required if the --verify-client is True in server endpoint.")
	rootCmd.PersistentFlags().BoolVar(&insecureSkipVerify, "insecure-skip-verify", false, "Whether skip verify server endpoint.")
	rootCmd.PersistentFlags().StringVar(
		&caFile, "ca-file", "",
		"The certificate authority file path, used to verify server endpoint certificate. If not specified, quictun try to load system certificate.")
	rootCmd.PersistentFlags().StringVar(&apiListenOn, "httpd-listen-on", "0.0.0.0:8086", "The socket of the API(httpd) server listen on.")
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

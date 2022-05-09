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
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/server"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	listenSocket      string
	keyFile           string
	certFile          string
	caFile            string
	verifyClient      bool
	tokenParserPlugin string
	tokenParserKey    string
)

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
	rootCmd := &cobra.Command{
		Use:   "quictun-server",
		Short: "Start up server side endpoint",
		Run: func(cmd *cobra.Command, args []string) {
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

			s := &server.ServerEndpoint{
				Address:     listenSocket,
				TlsConfig:   tlsConfig,
				TokenParser: loadTokenParserPlugin(tokenParserPlugin, tokenParserKey),
			}
			err := s.Start()
			if err != nil {
				klog.ErrorS(err, "Failed to start server endpoint")
			}
		},
	}
	defer klog.Flush()
	rootCmd.PersistentFlags().StringVar(&listenSocket, "listen-on", "0.0.0.0:7500", "The address that quic-tun server side endpoint listen on")
	rootCmd.PersistentFlags().StringVar(&tokenParserPlugin, "token-parser-plugin", "Cleartext", "The token parser plugin.")
	rootCmd.PersistentFlags().StringVar(&tokenParserKey, "token-parser-key", "", "An argument to be passed to the token parse plugin on instantiation.")
	rootCmd.PersistentFlags().StringVar(&keyFile, "key-file", "", "The private key file path.")
	rootCmd.PersistentFlags().StringVar(&certFile, "cert-file", "", "The certificate file path")
	rootCmd.PersistentFlags().BoolVar(&verifyClient, "verify-client", false, "Whether to require client certificate and verify it")
	rootCmd.PersistentFlags().StringVar(
		&caFile, "ca-file", "",
		"The certificate authority file path, used to verify client endpoint certificate. If not specified, quictun try to load system certificate.")
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

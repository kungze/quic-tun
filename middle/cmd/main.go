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
	"math/big"
	"os"

	"github.com/kungze/quic-tun/middle"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	middleOptions *options.MiddleOptions
	logOptions    *log.Options
)

func buildCommand(basename string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   basename,
		Short: "Start up the middle side endpoint",
		Long: `Establish a fast&security tunnel,
make you can access remote TCP/UNIX application like local application.

Find more quic-tun information at:
	https://github.com/kungze/quic-tun/blob/master/README.md`,
		RunE: runCommand,
	}
	// Initialize the flags needed to start the server
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	options.AddConfigFlag(basename, rootCmd.Flags())
	logOptions.AddFlags(rootCmd.Flags())
	middleOptions.AddFlags(rootCmd.Flags())

	return rootCmd
}

func runCommand(cmd *cobra.Command, args []string) error {
	options.PrintWorkingDir()
	options.PrintFlags(cmd.Flags())
	options.PrintConfig()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	if err := viper.Unmarshal(middleOptions); err != nil {
		return err
	}

	if err := viper.Unmarshal(logOptions); err != nil {
		return err
	}

	// run server
	runFunc(middleOptions)
	return nil
}

func runFunc(mo *options.MiddleOptions) {
	log.Init(logOptions)
	defer log.Flush()
	tlsConfig := generateTLSConfig()

	middle.NewMiddleEndpoint(mo, tlsConfig).Start()
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

func main() {
	// Initialize the options needed to start the server
	middleOptions = options.GetDefaultMiddleOptions()
	logOptions = log.NewOptions()

	rootCmd := buildCommand("quictun-middle")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

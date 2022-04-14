/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"os"

	"github.com/jeffyjf/quic-tun/server"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var listenSocket string

func main() {
	rootCmd := &cobra.Command{
		Use:   "quictun-server",
		Short: "Start up server side endpoint",
		Run: func(cmd *cobra.Command, args []string) {
			s := &server.ServerEndpoint{
				Address: listenSocket,
			}
			s.Start()
		},
	}
	defer klog.Flush()
	rootCmd.PersistentFlags().StringVar(&listenSocket, "listen-on", "0.0.0.0:7500", "The address that quic-tun server side endpoint listen on")
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

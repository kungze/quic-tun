/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"os"

	"github.com/jeffyjf/quic-tun/client"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var localSocket, serverEndpointSocket, token string

func main() {
	rootCmd := &cobra.Command{
		Use:   "quictun-client",
		Short: "Start up the client side endpoint",
		Run: func(cmd *cobra.Command, args []string) {
			c := client.ClientEndpoint{LocalSocket: localSocket, ServerEndpointSocket: serverEndpointSocket, Token: token}
			c.Start()
		},
	}
	defer klog.Flush()
	rootCmd.PersistentFlags().StringVar(&localSocket, "listen-on", "tcp:127.0.0.1:6500", "The socket that the client side endpoint listen on.")
	rootCmd.PersistentFlags().StringVar(&serverEndpointSocket, "server-endpoint", "", "The server side endpoint address, example: example.com:6565")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Used to tell the server endpoint which server app we want to connect.")
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

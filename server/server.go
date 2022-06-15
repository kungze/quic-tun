package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"strings"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/pkg/tunnel"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ServerEndpoint struct {
	Address     string
	TlsConfig   *tls.Config
	TokenParser token.TokenParserPlugin
}

func (s *ServerEndpoint) Start() {
	// Listen a quic(UDP) socket.
	listener, err := quic.ListenAddr(s.Address, s.TlsConfig, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	klog.InfoS("Server endpoint start up successful", "listen address", listener.Addr())
	for {
		// Wait client endpoint connection request.
		session, err := listener.Accept(context.Background())
		if err != nil {
			klog.ErrorS(err, "Encounter error when accept a connection.")
		} else {
			parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
			logger := klog.NewKlogr().WithValues(constants.ClientEndpointAddr, session.RemoteAddr().String())
			logger.Info("A new client endpoint connect request accepted.")
			go func() {
				for {
					// Wait client endpoint open a stream (A new steam means a new tunnel)
					stream, err := session.AcceptStream(context.Background())
					if err != nil {
						logger.Error(err, "Cannot accept a new stream.")
						break
					}
					logger := logger.WithValues(constants.StreamID, stream.StreamID())
					ctx := klog.NewContext(parent_ctx, logger)
					hsh := tunnel.NewHandshakeHelper(constants.AckMsgLength, handshake)
					hsh.TokenParser = &s.TokenParser

					tun := tunnel.NewTunnel(&stream, constants.ServerEndpoint)
					tun.Hsh = &hsh
					if !tun.HandShake(ctx) {
						continue
					}
					// After handshake successful the server application's address is established we can add it to log
					ctx = klog.NewContext(ctx, logger.WithValues(constants.ServerAppAddr, (*tun.Conn).RemoteAddr().String()))
					go tun.Establish(ctx)
				}
			}()
		}
	}
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper) (bool, *net.Conn) {
	logger := klog.FromContext(ctx)
	logger.Info("Starting handshake with client endpoint")
	if _, err := io.CopyN(hsh, *stream, constants.TokenLength); err != nil {
		logger.Error(err, "Can not receive token")
		return false, nil
	}
	addr, err := (*hsh.TokenParser).ParseToken(hsh.ReceiveData)
	if err != nil {
		logger.Error(err, "Failed to parse token")
		hsh.SetSendData([]byte{constants.ParseTokenError})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger = logger.WithValues(constants.ServerAppAddr, addr)
	logger.Info("starting connect to server app")
	sockets := strings.Split(addr, ":")
	conn, err := net.Dial(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		logger.Error(err, "Failed to dial server app")
		hsh.SetSendData([]byte{constants.CannotConnServer})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger.Info("Server app connect successful")
	hsh.SetSendData([]byte{constants.HandshakeSuccess})
	if _, err = io.CopyN(*stream, hsh, constants.AckMsgLength); err != nil {
		logger.Error(err, "Faied to send ack info", hsh.SendData)
		return false, nil
	}
	logger.Info("Handshake successful")
	return true, &conn
}

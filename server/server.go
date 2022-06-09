package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/handshake"
	"github.com/kungze/quic-tun/pkg/token"
	quic "github.com/lucas-clemente/quic-go"
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
		sess, err := listener.Accept(context.Background())
		if err != nil {
			klog.ErrorS(err, "Encounter error when accept a connection.")
		} else {
			logger := klog.NewKlogr().WithValues(constants.ClientEndpointAddr, sess.RemoteAddr().String())
			logger.Info("A new client endpoint connect request accepted.")
			go func() {
				for {
					// Wait client endpoint open a stream (A new steam means a new tunnel)
					stream, err := sess.AcceptStream(context.Background())
					if err != nil {
						logger.Error(err, "Cannot accept a new stream.")
						break
					}
					logger := logger.WithValues(constants.StreamID, stream.StreamID())
					go s.establishTunnel(logger, &stream)
				}
			}()
		}
	}
}

func (s *ServerEndpoint) establishTunnel(logger klog.Logger, stream *quic.Stream) {
	logger.Info("Starting establish a new tunnel.")
	defer func() {
		(*stream).Close()
		logger.Info("Tunnel closed")
	}()
	// Verify the token receive from client endpoint
	conn, err := s.handshake(logger, stream)
	if err != nil {
		return
	}
	defer conn.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	// Exchange packets between server application and client endpoint.
	go s.serverToClient(logger, &conn, stream, &wg)
	go s.clientToServer(logger, &conn, stream, &wg)
	logger.Info("Tunnel established")
	wg.Wait()
}

func (s *ServerEndpoint) handshake(logger klog.Logger, stream *quic.Stream) (net.Conn, error) {
	logger.Info("Starting handshake")
	hsh := handshake.NewHandshakeHelper([]byte{constants.HandshakeSuccess}, constants.AckMsgLength)
	if _, err := io.CopyN(&hsh, *stream, constants.TokenLength); err != nil {
		logger.Error(err, "Can not receive token")
		return nil, err
	}
	addr, err := s.TokenParser.ParseToken(hsh.ReceiveData)
	if err != nil {
		logger.Error(err, "Failed to parse token")
		hsh.SendData = []byte{constants.ParseTokenError}
		_, _ = io.Copy(*stream, &hsh)
		return nil, err
	}
	logger = logger.WithValues(constants.ServerAppAddr, addr)
	logger.Info("starting connect to server app")
	sockets := strings.Split(addr, ":")
	conn, err := net.Dial(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		logger.Error(err, "Failed to dial server app")
		hsh.SendData = []byte{constants.CannotConnServer}
		_, _ = io.Copy(*stream, &hsh)
		return nil, err
	}
	logger.Info("Server app connect successful")
	if _, err = io.CopyN(*stream, &hsh, constants.AckMsgLength); err != nil {
		logger.Error(err, "Faied to send ack info", hsh.SendData)
		return nil, err
	}
	logger.Info("Handshake successful")
	return conn, nil
}

func (s *ServerEndpoint) clientToServer(logger klog.Logger, server *net.Conn, client *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		(*client).Close()
		(*server).Close()
	}()
	_, err := io.Copy(*server, *client)
	if err != nil {
		logger.Error(err, "Can not forward packet from client to server")
	}
}

func (s *ServerEndpoint) serverToClient(logger klog.Logger, server *net.Conn, client *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		(*client).Close()
		(*server).Close()
	}()
	_, err := io.Copy(*client, *server)
	if err != nil {
		logger.Error(err, "Can not forward packet from server to client")
	}
}

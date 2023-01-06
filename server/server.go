package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"strings"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/log"
	nattraversal "github.com/kungze/quic-tun/pkg/nat-traversal"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/pkg/tunnel"
	"github.com/lucas-clemente/quic-go"
)

type ServerEndpoint struct {
	Address     string
	TlsConfig   *tls.Config
	TokenParser token.TokenParserPlugin
}

func (s *ServerEndpoint) Start(nt *options.NATTraversalOptions) {
	if nt.NATTraversalMode {
		for {
			connCtrl := nattraversal.NewConnCtrl(nt)
			// Subscribe and wait for messages from the remote peer
			connCtrl.MqttClient = nattraversal.NewMQTTClient(connCtrl.Nt, connCtrl.SdCh)
			nattraversal.Subscribe(connCtrl.MqttClient)
			log.Debug("wait for new client connection")
			connCtrl.RemoteSd = <-connCtrl.SdCh
			ctx, cancel := context.WithCancel(context.TODO())
			go func() {
				go nattraversal.ListenUDP(ctx, connCtrl)
				select {
				case <-connCtrl.ExitChan:
					log.Warn("first nat traversal failed, the second nat traversal attempt")
					go nattraversal.DialUDP(ctx, connCtrl)
					select {
					case <-connCtrl.ConvertExitChan:
						log.Warn("nat traversal faild!")
						cancel()
					case conn := <-connCtrl.ConnChan:
						log.Infof("nat traversal success! Remote address is %s", conn.Conn.RemoteAddr())
						go s.new(conn)
					}
				case conn := <-connCtrl.ConnChan:
					log.Info("nat traversal success!")
					go s.new(conn)
				}
			}()
		}
	} else {
		laddr, err := net.ResolveUDPAddr("udp", s.Address)
		if err != nil {
			panic(err)
		}
		conn, err := net.ListenUDP("udp", laddr)
		if err != nil {
			panic(err)
		}
		s.new(conn)
	}
}

func (s *ServerEndpoint) new(conn net.PacketConn) {
	// Listen a quic(UDP) socket.
	// listener, err := quic.ListenAddr(s.Address, s.TlsConfig, nil)
	listener, err := quic.Listen(conn, s.TlsConfig, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Infow("Server endpoint start up successful", "listen address", listener.Addr())
	for {
		// Wait client endpoint connection request.
		session, err := listener.Accept(context.Background())
		if err != nil {
			log.Errorw("Encounter error when accept a connection.", "error", err.Error())
		} else {
			parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
			logger := log.WithValues(constants.ClientEndpointAddr, session.RemoteAddr().String())
			logger.Info("A new client endpoint connect request accepted.")
			go func() {
				for {
					// Wait client endpoint open a stream (A new steam means a new tunnel)
					stream, err := session.AcceptStream(context.Background())
					if err != nil {
						logger.Errorw("Cannot accept a new stream.", "error", err.Error())
						break
					}
					logger := logger.WithValues(constants.StreamID, stream.StreamID())
					ctx := logger.WithContext(parent_ctx)
					hsh := tunnel.NewHandshakeHelper(constants.AckMsgLength, handshake)
					hsh.TokenParser = &s.TokenParser

					tun := tunnel.NewTunnel(&stream, constants.ServerEndpoint)
					tun.Hsh = &hsh
					if !tun.HandShake(ctx) {
						continue
					}
					// After handshake successful the server application's address is established we can add it to log
					ctx = logger.WithValues(constants.ServerAppAddr, (*tun.Conn).RemoteAddr().String()).WithContext(ctx)
					go tun.Establish(ctx)
				}
			}()
		}
	}
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper) (bool, *net.Conn) {
	logger := log.FromContext(ctx)
	logger.Info("Starting handshake with client endpoint")
	if _, err := io.CopyN(hsh, *stream, constants.TokenLength); err != nil {
		logger.Errorw("Can not receive token", "error", err.Error())
		return false, nil
	}
	addr, err := (*hsh.TokenParser).ParseToken(hsh.ReceiveData)
	if err != nil {
		logger.Errorw("Failed to parse token", "error", err.Error())
		hsh.SetSendData([]byte{constants.ParseTokenError})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger = logger.WithValues(constants.ServerAppAddr, addr)
	logger.Info("starting connect to server app")
	sockets := strings.Split(addr, ":")
	conn, err := net.Dial(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		logger.Errorw("Failed to dial server app", "error", err.Error())
		hsh.SetSendData([]byte{constants.CannotConnServer})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger.Info("Server app connect successful")
	hsh.SetSendData([]byte{constants.HandshakeSuccess})
	if _, err = io.CopyN(*stream, hsh, constants.AckMsgLength); err != nil {
		logger.Errorw("Faied to send ack info", "error", err.Error(), "", hsh.SendData)
		return false, nil
	}
	logger.Info("Handshake successful")
	return true, &conn
}

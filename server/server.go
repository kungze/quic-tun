package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/kungze/quic-tun/pkg/msg"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/pkg/tunnel"
	"github.com/lucas-clemente/quic-go"
)

type ServerEndpoint struct {
	Address        string
	TlsConfig      *tls.Config
	TokenParser    token.TokenParserPlugin
	MiddleEndpoint string
	SignKey        string
}

// PrepareStart used to complete the hole-punching operation before start quictun-server
func (s *ServerEndpoint) PrepareStart() error {
	var wg sync.WaitGroup
	wg.Add(1)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-tun"},
	}
	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Errorw("Failed to load system cert pool", "error", err.Error())
		return err
	}
	tlsConfig.ClientCAs = certPool

	// Dial middle endpoint
	session, err := quic.DialAddr(s.MiddleEndpoint, tlsConfig, &quic.Config{KeepAlive: true})
	if err != nil {
		panic(err)
	}
	log.Infow("Connection to middle endpoint successful", "local address", session.LocalAddr().String())

	// Open strem
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()
	log.Infow("Stream open successful", "stream ID", stream.StreamID())

	frameCodec := msg.NewFrameCodec()
	// Send msg
	ns := &msg.NatHoleQServer{
		SignKey: s.SignKey,
	}

	framePayload, err := msg.Encode(ns)
	if err != nil {
		panic(err)
	}

	err = frameCodec.Encode(stream, framePayload)
	if err != nil {
		panic(err)
	}

	// Receive msg
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Infof("Disconnect from middle: %v", err)
			}
		}()
		for {
			// handle ack
			// read from the stream
			ackFramePayLoad, err := frameCodec.Decode(stream)
			if err != nil {
				panic(err)
			}

			p, err := msg.Decode(ackFramePayLoad)
			if err != nil {
				panic(err)
			}

			switch p := p.(type) {
			case *msg.NatHoleResp:
				log.Infof("recv nathole resp: client public address is %s", p.QClientAddr)
				go func() {
					log.Info("Trying to punch a hole")
					localUDPAddr, _ := net.ResolveUDPAddr("udp", session.LocalAddr().String())
					remoteUDPAddr, _ := net.ResolveUDPAddr("udp", p.QClientAddr)

					// close quic conn
					log.Info("close the quictun-middle stream")
					stream.Close()
					err := session.CloseWithError(0, errors.New("close quic connection").Error())
					if err != nil {
						panic(err)
					}

					// start a udp conn
					conn, err := net.DialUDP("udp", localUDPAddr, remoteUDPAddr)
					if err != nil {
						log.Error(err.Error())
					}
					defer conn.Close()
					log.Infof("Real Dial UDP %s %s", conn.LocalAddr().String(), conn.RemoteAddr().String())

					log.Info("send nat hole first message to quictun-client")
					if _, err = conn.Write([]byte("server holeMsg")); err != nil {
						log.Infow("send nat hole message to client error", "err", err.Error())
					}

					err = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					if err != nil {
						panic(err)
					}

					data := make([]byte, 1024)
					n, _, err := conn.ReadFromUDP(data)
					if err != nil {
						log.Warnf("get nat hole message from quictun-client error: %v", err)
						return
					} else {
						log.Infof("get nat hole message from quictun-client successful, recv holeMsg: %s", data[:n])
					}
					s.Address = conn.LocalAddr().String()

					log.Info("send nat hole second message to quictun-client")
					if _, err = conn.Write([]byte("server holeMsg")); err != nil {
						log.Infow("send nat hole second message to quictun-client error", "err", err.Error())
					}

					wg.Done()
				}()
			default:
				log.Error("unknown packet type")
			}

		}
	}()

	wg.Wait()
	return nil
}

func (s *ServerEndpoint) Start() {
	// Listen a quic(UDP) socket.
	listener, err := quic.ListenAddr(s.Address, s.TlsConfig, nil)
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

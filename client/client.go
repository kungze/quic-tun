package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	TokenSource          token.TokenSourcePlugin
	TlsConfig            *tls.Config
	MiddleEndpoint       string
	SignKey              string
	NatHoleQSAddr        *net.UDPAddr
}

// PrepareStart used to complete the hole-punching operation before connection quictun-server
func (c *ClientEndpoint) PrepareStart() error {
	var wg sync.WaitGroup
	wg.Add(1)

	// Dial middle endpoint
	session, err := quic.DialAddr(c.MiddleEndpoint, c.TlsConfig, &quic.Config{KeepAlive: true})
	if err != nil {
		panic(err)
	}
	log.Infow("Connection to middle endpoint successful", "local address", session.LocalAddr().String())

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()
	log.Infow("Stream open successful", "stream ID", stream.StreamID())

	// Send msg
	frameCodec := msg.NewFrameCodec()
	nc := &msg.NatHoleQClient{
		SignKey: c.SignKey,
	}

	framePayload, err := msg.Encode(nc)
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
				log.Infof("Disconnect quictun-middle: %v", err)
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
				log.Infof("recv nathole resp: server public address is %s", p.QServerAddr)
				go func() {
					log.Info("Trying to punch a hole")
					localUDPAddr, _ := net.ResolveUDPAddr("udp", session.LocalAddr().String())
					remoteUDPAddr, _ := net.ResolveUDPAddr("udp", p.QServerAddr)

					// close quic conn
					log.Info("close the quictun-middle connection")
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
					log.Infof("Dial UDP %s %s", conn.LocalAddr().String(), conn.RemoteAddr().String())

					// wait for quictun-server start udp listen
					time.Sleep(1 * time.Second)
					log.Info("send nat hole first message to quictun-server")
					if _, err = conn.Write([]byte("client holeMsg")); err != nil {
						log.Errorw("send nat hole message to server error", "err", err.Error())
					}

					// start send nat hole msg
					err = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
					if err != nil {
						panic(err)
					}

					data := make([]byte, 1024)
					n, _, err := conn.ReadFromUDP(data)
					if err != nil {
						log.Warnf("get nat hole message from quictun-server error: %v", err)
					} else {
						log.Infof("get nat hole message from quictun-server successful, recv holeMsg: %s", data[:n])
					}
					c.ServerEndpointSocket = p.QServerAddr
					c.NatHoleQSAddr = localUDPAddr
					// wait for quictun-server start quic listen
					time.Sleep(1 * time.Second)

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

func (c *ClientEndpoint) Start() {
	udpConn, err := net.ListenUDP("udp", c.NatHoleQSAddr)
	if err != nil {
		panic(err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", c.ServerEndpointSocket)
	if err != nil {
		panic(err)
	}
	// Dial server endpoint
	session, err := quic.Dial(udpConn, udpAddr, c.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlive: true})
	log.Infow("Connection server endpoint successful", "local address", session.LocalAddr().String())
	if err != nil {
		panic(err)
	}
	parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
	// Listen on a TCP or UNIX socket, wait client application's connection request.
	localSocket := strings.Split(c.LocalSocket, ":")
	listener, err := net.Listen(strings.ToLower(localSocket[0]), strings.Join(localSocket[1:], ":"))
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Infow("Client endpoint start up successful", "listen address", listener.Addr())
	for {
		// Accept client application connectin request
		conn, err := listener.Accept()
		if err != nil {
			log.Errorw("Client app connect failed", "error", err.Error())
		} else {
			logger := log.WithValues(constants.ClientAppAddr, conn.RemoteAddr().String())
			logger.Info("Client connection accepted, prepare to entablish tunnel with server endpint for this connection.")
			go func() {
				defer func() {
					conn.Close()
					logger.Info("Tunnel closed")
				}()
				// Open a quic stream for each client application connection.
				stream, err := session.OpenStreamSync(context.Background())
				if err != nil {
					logger.Errorw("Failed to open stream to server endpoint.", "error", err.Error())
					return
				}
				defer stream.Close()
				logger = logger.WithValues(constants.StreamID, stream.StreamID())
				// Create a context argument for each new tunnel
				ctx := context.WithValue(
					logger.WithContext(parent_ctx),
					constants.CtxClientAppAddr, conn.RemoteAddr().String())
				hsh := tunnel.NewHandshakeHelper(constants.TokenLength, handshake)
				hsh.TokenSource = &c.TokenSource
				// Create a new tunnel for the new client application connection.
				tun := tunnel.NewTunnel(&stream, constants.ClientEndpoint)
				tun.Conn = &conn
				tun.Hsh = &hsh
				if !tun.HandShake(ctx) {
					return
				}
				tun.Establish(ctx)
			}()
		}
	}
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper) (bool, *net.Conn) {
	logger := log.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token, err := (*hsh.TokenSource).GetToken(fmt.Sprint(ctx.Value(constants.CtxClientAppAddr)))
	if err != nil {
		logger.Errorw("Encounter error.", "erros", err.Error())
		return false, nil
	}
	hsh.SetSendData([]byte(token))
	_, err = io.CopyN(*stream, hsh, constants.TokenLength)
	if err != nil {
		logger.Errorw("Failed to send token", err.Error())
		return false, nil
	}
	_, err = io.CopyN(hsh, *stream, constants.AckMsgLength)
	if err != nil {
		logger.Errorw("Failed to receive ack", err.Error())
		return false, nil
	}
	switch hsh.ReceiveData[0] {
	case constants.HandshakeSuccess:
		logger.Info("Handshake successful")
		return true, nil
	case constants.ParseTokenError:
		logger.Errorw("handshake error!", "error", "server endpoint can not parser token")
		return false, nil
	case constants.CannotConnServer:
		logger.Errorw("handshake error!", "error", "server endpoint can not connect to server application")
		return false, nil
	default:
		logger.Errorw("handshake error!", "error", "received an unknow ack info")
		return false, nil
	}
}

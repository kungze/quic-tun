package client

import (
	"context"
	"crypto/tls"
	"fmt"
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

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	TokenSource          token.TokenSourcePlugin
	TlsConfig            *tls.Config
}

func (c *ClientEndpoint) Start(nt *options.NATTraversalOptions) {
	if nt.NATTraversalMode {
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		connCtrl := nattraversal.NewConnCtrl(nt)
		go nattraversal.DialUDP(ctx, connCtrl)
		select {
		case <-connCtrl.ExitChan:
			log.Warn("first nat traversal failed, the second nat traversal attempt")
			// Subscribe and wait for messages from the remote peer
			connCtrl.MqttClient = nattraversal.NewMQTTClient(connCtrl.Nt, connCtrl.SdCh)
			nattraversal.Subscribe(connCtrl.MqttClient)
			connCtrl.RemoteSd = <-connCtrl.SdCh
			go nattraversal.ListenUDP(ctx, connCtrl)
			select {
			case <-connCtrl.ConvertExitChan:
				log.Warn("nat traversal faild!")
				cancel()
			case conn := <-connCtrl.ConnChan:
				log.Info("nat traversal success!")
				c.new(conn, conn.Conn.RemoteAddr(), conn.Conn.LocalAddr().String())
			}
		case conn := <-connCtrl.ConnChan:
			log.Infof("nat traversal success! Remote address is %s", conn.Conn.RemoteAddr())
			c.new(conn, conn.Conn.RemoteAddr(), conn.Conn.LocalAddr().String())
		}
	} else {
		raddr, err := net.ResolveUDPAddr("udp", c.ServerEndpointSocket)
		if err != nil {
			panic(err)
		}
		conn, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			panic(err)
		}
		c.new(conn, raddr, c.ServerEndpointSocket)
	}
}

func (c *ClientEndpoint) new(conn net.PacketConn, raddr net.Addr, host string) {
	// Dial server endpoint
	// quic.DialAddr()
	session, err := quic.Dial(conn, raddr, host, c.TlsConfig, &quic.Config{KeepAlive: true})
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

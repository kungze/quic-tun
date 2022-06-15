package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/kungze/quic-tun/pkg/tunnel"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	TokenSource          token.TokenSourcePlugin
	TlsConfig            *tls.Config
}

func (c *ClientEndpoint) Start() {
	// Dial server endpoint
	session, err := quic.DialAddr(c.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlive: true})
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
	klog.InfoS("Client endpoint start up successful", "listen address", listener.Addr())
	for {
		// Accept client application connectin request
		conn, err := listener.Accept()
		if err != nil {
			klog.ErrorS(err, "Client app connect failed")
		} else {
			logger := klog.NewKlogr().WithValues(constants.ClientAppAddr, conn.RemoteAddr().String())
			logger.Info("Client connection accepted, prepare to entablish tunnel with server endpint for this connection.")
			go func() {
				defer func() {
					conn.Close()
					logger.Info("Tunnel closed")
				}()
				// Open a quic stream for each client application connection.
				stream, err := session.OpenStreamSync(context.Background())
				if err != nil {
					logger.Error(err, "Failed to open stream to server endpoint.")
					return
				}
				defer stream.Close()
				logger = logger.WithValues(constants.StreamID, stream.StreamID())
				// Create a context argument for each new tunnel
				ctx := context.WithValue(
					klog.NewContext(parent_ctx, logger),
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
	logger := klog.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token, err := (*hsh.TokenSource).GetToken(fmt.Sprint(ctx.Value(constants.CtxClientAppAddr)))
	if err != nil {
		logger.Error(err, "Encounter error.")
		return false, nil
	}
	hsh.SetSendData([]byte(token))
	_, err = io.CopyN(*stream, hsh, constants.TokenLength)
	if err != nil {
		logger.Error(err, "Failed to send token")
		return false, nil
	}
	_, err = io.CopyN(hsh, *stream, constants.AckMsgLength)
	if err != nil {
		logger.Error(err, "Failed to receive ack")
		return false, nil
	}
	switch hsh.ReceiveData[0] {
	case constants.HandshakeSuccess:
		logger.Info("Handshake successful")
		return true, nil
	case constants.ParseTokenError:
		logger.Error(errors.New("Server endpoint can not parser token."), "Handshake error!")
		return false, nil
	case constants.CannotConnServer:
		logger.Error(errors.New("Server endpoint can not connect to server application."), "Handshake error!")
		return false, nil
	default:
		logger.Error(errors.New("Received an unknow ack info."), "Handshake error!")
		return false, nil
	}
}

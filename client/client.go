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

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/handshake"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	TokenSource          token.TokenSourcePlugin
	TlsConfig            *tls.Config
}

func (c *ClientEndpoint) Start() error {
	sockets := strings.Split(c.LocalSocket, ":")
	listener, err := net.Listen(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		klog.ErrorS(err, "Failed to start up")
		return err
	}
	defer listener.Close()
	klog.InfoS("Client endpoint start up successful", "listen address", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			klog.ErrorS(err, "Client app connect failed")
		} else {
			logger := klog.NewKlogr().WithValues("Client-App-Addr", conn.RemoteAddr().String())
			ctx := context.WithValue(klog.NewContext(context.TODO(), logger), "client-app-addr", conn.RemoteAddr().String())
			logger.Info("Accepted a client connection")
			go func() {
				defer func() {
					conn.Close()
					logger.Info("Tunnel closed")
				}()
				c.establishTunnel(ctx, &conn)
			}()

		}
	}
}

func (c *ClientEndpoint) establishTunnel(ctx context.Context, conn *net.Conn) {
	logger := klog.FromContext(ctx)
	logger.Info("Establishing a new tunnel.")
	session, err := quic.DialAddr(c.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlive: true})
	if err != nil {
		logger.Error(err, "Failed to dial server endpoint.")
		return
	}
	logger = logger.WithValues("Local-Addr", session.LocalAddr().String())
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		logger.Error(err, "Failed to open stream to server endpoint.")
		return
	}
	defer stream.Close()
	err = c.handshake(ctx, &stream)
	if err != nil {
		logger.Error(err, "Handshake failed.")
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go c.clientToServer(logger, conn, &stream, &wg)
	go c.serverToClient(logger, conn, &stream, &wg)
	logger.Info("Tunnel established")
	wg.Wait()
}

func (c *ClientEndpoint) handshake(ctx context.Context, stream *quic.Stream) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token, err := c.TokenSource.GetToken(fmt.Sprint(ctx.Value("client-app-addr")))
	if err != nil {
		logger.Error(err, "Encounter error.")
		return err
	}
	hsh := handshake.NewHandshakeHelper([]byte(token), constants.TokenLength)
	_, err = io.CopyN(*stream, &hsh, constants.TokenLength)
	if err != nil {
		logger.Error(err, "Failed to send token")
		return err
	}
	_, err = io.CopyN(&hsh, *stream, constants.AckMsgLength)
	if err != nil {
		logger.Error(err, "Failed to receive ack")
		return err
	}
	switch hsh.ReceiveData[0] {
	case constants.HandshakeSuccess:
		logger.Info("Handshake successful")
		return nil
	case constants.ParserTokenError:
		return errors.New("Server endpoint can not parser token.")
	case constants.CannotConnServer:
		return errors.New("Server endpoint can not connect to server application.")
	default:
		logger.Info("Received an unkone ack info", "ack", hsh.ReceiveData)
		return errors.New("Handshake error! Received an unkone ack info.")
	}
}

func (c *ClientEndpoint) clientToServer(logger klog.Logger, client *net.Conn, server *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		(*client).Close()
		(*server).Close()
		wg.Done()
	}()
	_, err := io.Copy(*server, *client)
	if err != nil {
		logger.Error(err, "Can not forward packet from client to server")
	}
}

func (c *ClientEndpoint) serverToClient(logger klog.Logger, client *net.Conn, server *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		(*client).Close()
		(*server).Close()
		wg.Done()
	}()
	_, err := io.Copy(*client, *server)
	if err != nil {
		logger.Error(err, "Can not forward packet from server to client")
	}
}

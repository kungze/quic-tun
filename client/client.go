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

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/handshake"
	"github.com/kungze/quic-tun/pkg/restfulapi"
	"github.com/kungze/quic-tun/pkg/token"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	TokenSource          token.TokenSourcePlugin
	TlsConfig            *tls.Config
	// Used to send tunnel status info to httpd(API) server
	TunCh chan<- restfulapi.Tunnel
}

func (c *ClientEndpoint) Start() {
	// Dial server endpoint
	session, err := quic.DialAddr(c.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlive: true})
	if err != nil {
		panic(err)
	}
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
				ctx := context.WithValue(klog.NewContext(context.TODO(), logger), constants.CtxClientAppAddrKey, conn.RemoteAddr().String())
				tunnelData := restfulapi.Tunnel{
					Uuid:               uuid.New(),
					StreamID:           stream.StreamID(),
					ClientAppAddr:      conn.RemoteAddr().String(),
					RemoteEndpointAddr: session.RemoteAddr().String(),
				}
				c.establishTunnel(ctx, &conn, &stream, &tunnelData)
			}()
		}
	}
}

func (c *ClientEndpoint) establishTunnel(ctx context.Context, conn *net.Conn, stream *quic.Stream, tunnelData *restfulapi.Tunnel) {
	logger := klog.FromContext(ctx)
	logger.Info("Establishing a new tunnel.")
	// Sent token to server endpoint
	err := c.handshake(ctx, stream)
	if err != nil {
		logger.Error(err, "Handshake failed.")
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	// Exchange packets between server endpoint and client application.
	go c.clientToServer(logger, conn, stream, &wg)
	go c.serverToClient(logger, conn, stream, &wg)
	logger.Info("Tunnel established")
	// Notify httpd(API) server a new tunnel was created
	tunnelData.CreatedAt = time.Now().String()
	tunnelData.Action = constants.Creation
	c.TunCh <- *tunnelData
	wg.Wait()
	tunnelData.Action = constants.Close
	c.TunCh <- *tunnelData
}

func (c *ClientEndpoint) handshake(ctx context.Context, stream *quic.Stream) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token, err := c.TokenSource.GetToken(fmt.Sprint(ctx.Value(constants.CtxClientAppAddrKey)))
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
	case constants.ParseTokenError:
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

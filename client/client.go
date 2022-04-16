package client

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/jeffyjf/quic-tun/pkg/constants"
	"github.com/jeffyjf/quic-tun/pkg/handshake"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ClientEndpoint struct {
	LocalSocket          string
	ServerEndpointSocket string
	Token                string
}

func (c *ClientEndpoint) Start() error {
	sockets := strings.Split(c.LocalSocket, ":")
	listener, err := net.Listen(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		klog.ErrorS(err, "Failed to start up")
		return err
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			klog.ErrorS(err, "Client app connect failed")
			return err
		}
		klog.InfoS("Accepted a client connection", "client app addr", conn.RemoteAddr().String())
		go c.establishTunnel(&conn)
	}
}

func (c *ClientEndpoint) clientToServer(client *net.Conn, server *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		(*client).Close()
		(*server).Close()
		wg.Done()
	}()
	_, err := io.Copy(*server, *client)
	if err != nil {
		klog.ErrorS(err, "Can not forward packet from client to server")
	}
}

func (c *ClientEndpoint) serverToClient(client *net.Conn, server *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		(*client).Close()
		(*server).Close()
		wg.Done()
	}()
	_, err := io.Copy(*client, *server)
	if err != nil {
		klog.ErrorS(err, "Can not forward packet from server to client")
	}
}

func (c *ClientEndpoint) establishTunnel(conn *net.Conn) {
	defer func() {
		(*conn).Close()
		klog.InfoS("Tunnel closed", "client app", (*conn).RemoteAddr())
	}()
	klog.Info("Establishing a new tunnel", "remote", c.ServerEndpointSocket)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-tun"},
	}
	session, err := quic.DialAddr(c.ServerEndpointSocket, tlsConf, &quic.Config{KeepAlive: true})
	if err != nil {
		klog.ErrorS(err, "Failed to dial server endpoint")
		return
	}
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		klog.ErrorS(err, "Failed to open stream to server endpoint")
		return
	}
	defer stream.Close()
	err = c.handshake(&stream)
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go c.clientToServer(conn, &stream, &wg)
	go c.serverToClient(conn, &stream, &wg)
	klog.InfoS("Tunnel established", "server endpoint", c.ServerEndpointSocket)
	wg.Wait()
}

func (c *ClientEndpoint) handshake(stream *quic.Stream) error {
	klog.InfoS("Startinig handshake with server endpoint", "token", c.Token)
	hsh := handshake.NewHandshakeHelper([]byte(c.Token), constants.TokenLength)
	_, err := io.CopyN(*stream, &hsh, constants.TokenLength)
	if err != nil {
		klog.ErrorS(err, "Failed to send token")
		return err
	}
	_, err = io.CopyN(&hsh, *stream, constants.AckMsgLength)
	if err != nil {
		klog.ErrorS(err, "Failed to receive ack")
		return err
	}
	switch hsh.ReceiveData[0] {
	case constants.HandshakeSuccess:
		klog.Info("Handshake successful")
		return nil
	default:
		klog.InfoS("Received an unkone ack info", "ack", hsh.ReceiveData)
		return errors.New("Handshake error! Received an unkone ack info.")
	}
}

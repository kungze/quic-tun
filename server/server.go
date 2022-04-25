package server

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/handshake"
	quic "github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type ServerEndpoint struct {
	Address   string
	TlsConfig *tls.Config
}

func (s *ServerEndpoint) Start() error {
	listener, err := quic.ListenAddr(s.Address, s.TlsConfig, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	klog.InfoS("Server endpoint start up successful", "listen address", listener.Addr())
	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			klog.ErrorS(err, "Encounter error when accept connection.")
			return err
		}
		klog.Info("Received a new connect request", "client endpoint", sess.RemoteAddr().String())
		go s.establishTunnel(&sess)
	}
}

func (s *ServerEndpoint) establishTunnel(sess *quic.Session) {
	remoteAddr := (*sess).RemoteAddr().String()
	klog.InfoS("Starting establish a new tunnel", "client endpoint", remoteAddr)
	stream, err := (*sess).AcceptStream(context.Background())
	if err != nil {
		klog.ErrorS(err, "Failed to accept stream", "client endpoint", remoteAddr)
		return
	}
	defer func() {
		stream.Close()
		klog.InfoS("Tunnel closed", "client endpoint", remoteAddr)
	}()
	conn, err := s.handshake(&stream)
	if err != nil {
		return
	}
	defer conn.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go s.serverToClient(&conn, &stream, &wg)
	go s.clientToServer(&conn, &stream, &wg)
	klog.InfoS("Tunnel established", "client endpoint", remoteAddr)
	wg.Wait()
}

func (s *ServerEndpoint) handshake(stream *quic.Stream) (net.Conn, error) {
	klog.Info("Starting handshake")
	hsh := handshake.NewHandshakeHelper([]byte{constants.HandshakeSuccess}, constants.AckMsgLength)
	_, err := io.CopyN(&hsh, *stream, constants.TokenLength)
	if err != nil {
		klog.ErrorS(err, "Can not receive token")
		return nil, err
	}
	klog.InfoS("starting connect to server app", "server app", hsh.ReceiveData)
	conn, err := net.Dial("tcp", hsh.ReceiveData)
	if err != nil {
		klog.ErrorS(err, "Failed to dial server app", "server address", hsh.ReceiveData)
		hsh.SendData = []byte{constants.HandshakeFailure}
		_, err = io.Copy(*stream, &hsh)
		if err != nil {
			klog.ErrorS(err, "Failed to dial server app", "server address", hsh.ReceiveData)
		}
		return nil, err
	}
	klog.Info("Server app connect successful")
	_, err = io.CopyN(*stream, &hsh, constants.AckMsgLength)
	if err != nil {
		klog.ErrorS(err, "Faied to send ack info", hsh.SendData)
		return nil, err
	}
	klog.Info("Handshake successful")
	return conn, nil
}

func (s *ServerEndpoint) clientToServer(server *net.Conn, client *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		(*client).Close()
		(*server).Close()
	}()
	_, err := io.Copy(*server, *client)
	if err != nil {
		klog.ErrorS(err, "Can not forward packet from client to server")
	}
}

func (s *ServerEndpoint) serverToClient(server *net.Conn, client *quic.Stream, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		(*client).Close()
		(*server).Close()
	}()
	_, err := io.Copy(*client, *server)
	if err != nil {
		klog.ErrorS(err, "Can not forward packet from server to client")
	}
}

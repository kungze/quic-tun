package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/kungze/quic-tun/internal/client/config"
	"github.com/kungze/quic-tun/internal/pkg/constants"
	"github.com/kungze/quic-tun/internal/pkg/options"
	"github.com/kungze/quic-tun/internal/pkg/restfulapi"
	"github.com/kungze/quic-tun/internal/pkg/token"
	"github.com/kungze/quic-tun/internal/pkg/tunnel"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/lucas-clemente/quic-go"
)

type clientEndpoint struct {
	ClientOptions     *options.ClientOptions
	RestfulAPIOptions *options.RestfulAPIOptions
	TokenSource       token.TokenSourcePlugin
	TlsConfig         *tls.Config
}

type preparedClient struct {
	*clientEndpoint
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper) (bool, *net.Conn) {
	logger := log.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token, err := (*hsh.TokenSource).GetToken(fmt.Sprint(ctx.Value(constants.CtxClientAppAddr)))
	if err != nil {
		logger.Errorf("Encounter error: %s", err.Error())
		return false, nil
	}
	hsh.SetSendData([]byte(token))
	_, err = io.CopyN(*stream, hsh, constants.TokenLength)
	if err != nil {
		logger.Errorf("Failed to send token: %s", err.Error())
		return false, nil
	}
	_, err = io.CopyN(hsh, *stream, constants.AckMsgLength)
	if err != nil {
		logger.Errorf("Failed to receive ack: %s", err.Error())
		return false, nil
	}
	switch hsh.ReceiveData[0] {
	case constants.HandshakeSuccess:
		logger.Info("Handshake successful")
		return true, nil
	case constants.ParseTokenError:
		logger.Errorw("Server endpoint can not parser token.", "Handshake error!")
		return false, nil
	case constants.CannotConnServer:
		logger.Errorw("Server endpoint can not connect to server application.", "Handshake error!")
		return false, nil
	default:
		logger.Errorw("Received an unknow ack info.", "Handshake error!")
		return false, nil
	}
}

func createClientEndpoint(cfg *config.Config) (*clientEndpoint, error) {

	keyFile := cfg.ClientOptions.KeyFile
	certFile := cfg.ClientOptions.CertFile
	insecureSkipVerify := !cfg.ClientOptions.VerifyServer
	caFile := cfg.ClientOptions.CaFile
	tokenParserPlugin := cfg.ClientOptions.TokenPlugin
	tokenParserKey := cfg.ClientOptions.TokenSource

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		NextProtos:         []string{"quic-tun"},
	}
	if certFile != "" && keyFile != "" {
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Errorf("Certificate file or private key file is invalid: %s", err.Error())
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}
	if caFile != "" {
		caPemBlock, err := os.ReadFile(caFile)
		if err != nil {
			log.Errorf("Failed to read ca file: %s", err.Error())
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caPemBlock)
		tlsConfig.RootCAs = certPool
	} else {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			log.Errorf("Failed to load system cert pool: %s", err.Error())
			return nil, err
		}
		tlsConfig.ClientCAs = certPool
	}

	// define client endpoint
	client := &clientEndpoint{
		ClientOptions:     cfg.ClientOptions,
		RestfulAPIOptions: cfg.RestfulAPIOptions,
		TlsConfig:         tlsConfig,
		TokenSource:       loadTokenSourcePlugin(tokenParserPlugin, tokenParserKey),
	}

	return client, nil
}

func (s *clientEndpoint) PrepareRun() preparedClient {
	return preparedClient{s}
}

func (c preparedClient) Run() error {
	address := c.RestfulAPIOptions.Address()
	// Start API server
	httpd := restfulapi.NewHttpd(address)
	go httpd.Start()

	// Dial server endpoint
	session, err := quic.DialAddr(c.ClientOptions.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlivePeriod: 15 * time.Second})
	if err != nil {
		panic(err)
	}
	parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
	// Listen on a TCP or UNIX socket, wait client application's connection request.
	listener, err := net.Listen(c.ClientOptions.BindProtocol, c.ClientOptions.Address())
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Infow("Client endpoint start up successful", "listen address", listener.Addr())
	for {
		// Accept client application connectin request
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("Client app connect failed: %s", err.Error())
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
					logger.Errorf("Failed to open stream to server endpoint: %s", err.Error())
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

func loadTokenSourcePlugin(plugin string, source string) token.TokenSourcePlugin {
	switch strings.ToLower(plugin) {
	case "fixed":
		return token.NewFixedTokenPlugin(source)
	case "file":
		return token.NewFileTokenSourcePlugin(source)
	default:
		panic(fmt.Sprintf("The token source plugin %s is invalid", plugin))
	}
}

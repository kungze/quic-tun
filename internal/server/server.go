package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"strings"

	"github.com/kungze/quic-tun/internal/pkg/constants"
	"github.com/kungze/quic-tun/internal/pkg/options"
	"github.com/kungze/quic-tun/internal/pkg/restfulapi"
	"github.com/kungze/quic-tun/internal/pkg/token"
	"github.com/kungze/quic-tun/internal/pkg/tunnel"
	"github.com/kungze/quic-tun/internal/server/config"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/lucas-clemente/quic-go"
)

type serverEndpoint struct {
	ServerOptions     *options.ServerOptions
	RestfulAPIOptions *options.RestfulAPIOptions
	TokenParser       token.TokenParserPlugin
	TlsConfig         *tls.Config
}

type preparedServer struct {
	*serverEndpoint
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper) (bool, *net.Conn) {
	logger := log.FromContext(ctx)
	logger.Info("Starting handshake with client endpoint")
	if _, err := io.CopyN(hsh, *stream, constants.TokenLength); err != nil {
		logger.Errorf("Can not receive token: %s", err.Error())
		return false, nil
	}
	addr, err := (*hsh.TokenParser).ParseToken(hsh.ReceiveData)
	if err != nil {
		logger.Errorf("Failed to parse token: %s", err.Error())
		hsh.SetSendData([]byte{constants.ParseTokenError})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger = logger.WithValues(constants.ServerAppAddr, addr)
	logger.Info("starting connect to server app")
	sockets := strings.Split(addr, ":")
	conn, err := net.Dial(strings.ToLower(sockets[0]), strings.Join(sockets[1:], ":"))
	if err != nil {
		logger.Errorf("Failed to dial server app: %s", err.Error())
		hsh.SetSendData([]byte{constants.CannotConnServer})
		_, _ = io.Copy(*stream, hsh)
		return false, nil
	}
	logger.Info("Server app connect successful")
	hsh.SetSendData([]byte{constants.HandshakeSuccess})
	if _, err = io.CopyN(*stream, hsh, constants.AckMsgLength); err != nil {
		logger.Errorf("Faied to send ack info %s: %s", hsh.SendData, err.Error())
		return false, nil
	}
	logger.Info("Handshake successful")
	return true, &conn
}

func createServerEndpoint(cfg *config.Config) (*serverEndpoint, error) {

	keyFile := cfg.ServerOptions.KeyFile
	certFile := cfg.ServerOptions.CertFile
	verifyClient := cfg.ServerOptions.VerifyClient
	caFile := cfg.ServerOptions.CaFile
	tokenParserPlugin := cfg.ServerOptions.TokenParserPlugin
	tokenParserKey := cfg.ServerOptions.TokenParserKey

	var tlsConfig *tls.Config
	if keyFile == "" || certFile == "" {
		tlsConfig = generateTLSConfig()
	} else {
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Errorf("Certificate file or private key file is invalid: %s", err.Error())
			return nil, err
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			NextProtos:   []string{"quic-tun"},
		}
	}
	if verifyClient {
		if caFile == "" {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				log.Errorf("Failed to load system cert pool: %s", err.Error())
				return nil, err
			}
			tlsConfig.ClientCAs = certPool
		} else {
			caPemBlock, err := os.ReadFile(caFile)
			if err != nil {
				log.Errorf("Failed to read ca file: %s", err.Error())
			}
			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(caPemBlock)
			tlsConfig.ClientCAs = certPool
		}
	}

	// define server endpoint
	server := &serverEndpoint{
		ServerOptions:     cfg.ServerOptions,
		RestfulAPIOptions: cfg.RestfulAPIOptions,
		TlsConfig:         tlsConfig,
		TokenParser:       loadTokenParserPlugin(tokenParserPlugin, tokenParserKey),
	}

	return server, nil
}

func (s *serverEndpoint) PrepareRun() preparedServer {
	return preparedServer{s}
}

func (s preparedServer) Run() error {
	address := s.RestfulAPIOptions.Address()
	// Start API server
	httpd := restfulapi.NewHttpd(address)
	go httpd.Start()

	// Listen a quic(UDP) socket.
	listener, err := quic.ListenAddr(s.serverEndpoint.ServerOptions.Address(), s.serverEndpoint.TlsConfig, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Infow("Server endpoint start up successful", "listen address", listener.Addr())
	for {
		// Wait client endpoint connection request.
		session, err := listener.Accept(context.Background())
		if err != nil {
			log.Errorf("Encounter error when accept a connection: %s", err.Error())
		} else {
			parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
			logger := log.WithValues(constants.ClientEndpointAddr, session.RemoteAddr().String())
			logger.Info("A new client endpoint connect request accepted.")
			go func() {
				for {
					// Wait client endpoint open a stream (A new steam means a new tunnel)
					stream, err := session.AcceptStream(context.Background())
					if err != nil {
						logger.Errorf("Cannot accept a new stream: %s", err.Error())
						break
					}
					logger := logger.WithValues(constants.StreamID, stream.StreamID())
					ctx := log.WithContext(parent_ctx)
					hsh := tunnel.NewHandshakeHelper(constants.AckMsgLength, handshake)
					hsh.TokenParser = &s.TokenParser

					tun := tunnel.NewTunnel(&stream, constants.ServerEndpoint)
					tun.Hsh = &hsh
					if !tun.HandShake(ctx) {
						continue
					}
					// After handshake successful the server application's address is established we can add it to log
					newLogger := logger.WithValues(constants.ServerAppAddr, (*tun.Conn).RemoteAddr().String())
					ctx = newLogger.WithContext(ctx)
					go tun.Establish(ctx)
				}
			}()
		}
	}
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-tun"},
	}
}

func loadTokenParserPlugin(plugin string, key string) token.TokenParserPlugin {
	switch strings.ToLower(plugin) {
	case "cleartext":
		return token.NewCleartextTokenParserPlugin(key)
	default:
		panic(fmt.Sprintf("Token parser plugin %s don't support", plugin))
	}
}

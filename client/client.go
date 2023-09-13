package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	ClientOpitons        options.ClientOptions
	ListenerByPort       map[int]*net.Listener
	FileTokenType        string
}

func (c *ClientEndpoint) Start(nt *options.NATTraversalOptions) {
	if nt.NATTraversalMode {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctrl := nattraversal.NewConnCtrl(nt)
		go nattraversal.DialUDP(ctx, ctrl)

		select {
		case <-ctrl.ExitChan:
			log.Warn("first nat traversal failed, the second nat traversal attempt")
			ctrl.MqttClient = nattraversal.NewMQTTClient(ctrl.Nt, ctrl.SdCh)
			nattraversal.Subscribe(ctrl.MqttClient)
			ctrl.RemoteSd = <-ctrl.SdCh
			go nattraversal.ListenUDP(ctx, ctrl)

			select {
			case <-ctrl.ConvertExitChan:
				log.Warn("nat traversal faild!")
			case conn := <-ctrl.ConnChan:
				log.Info("nat traversal success!")
				c.new(conn, conn.Conn.RemoteAddr(), conn.Conn.LocalAddr().String(), nil)
			}
		case conn := <-ctrl.ConnChan:
			log.Infof("nat traversal success! Remote address is %s", conn.Conn.RemoteAddr())
			c.new(conn, conn.Conn.RemoteAddr(), conn.Conn.LocalAddr().String(), nil)
		}
	} else {
		session, err := quic.DialAddr(c.ServerEndpointSocket, c.TlsConfig, &quic.Config{KeepAlive: true})
		if err != nil {
			panic(err)
		}
		c.new(nil, nil, "", session)
	}
}

func (c *ClientEndpoint) new(conn net.PacketConn, raddr net.Addr, host string, session quic.Session) {
	// Dial server endpoint
	if session == nil {
		ns, err := quic.Dial(conn, raddr, host, c.TlsConfig, &quic.Config{KeepAlive: true})
		if err != nil {
			panic(err)
		}
		session = ns
	}
	parent_ctx, cancle := context.WithCancel(context.TODO())
	defer cancle()
	conn_ctx := context.WithValue(parent_ctx, constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())

	ports := []int{}

	// Parse the token source if the token plugin is "file".
	if strings.ToLower(c.ClientOpitons.TokenPlugin) == "file" {
		file, err := os.Open(c.ClientOpitons.TokenSource)
		if err != nil {
			panic("error")
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			items := strings.Split(line, " ")
			if len(items) != 2 {
				continue
			}
			ipOrPort := strings.Split(items[0], ".")
			if len(ipOrPort) == 4 {
				if c.FileTokenType == "" {
					c.FileTokenType = constants.FileTokenTypeAddress
				}
				log.Debugf("%s is IP address", ipOrPort[0])
				portNum, err := getPortNumber(c.LocalSocket)
				if err != nil {
					panic("error")
				}
				ports = addPortNumber(ports, portNum)
				break
			} else if len(ipOrPort) == 1 {
				if c.FileTokenType == "" {
					c.FileTokenType = constants.FileTokenTypePort
				}
				log.Debugf("%s is port", ipOrPort[0])
				portNum, err := strconv.Atoi(ipOrPort[0])
				if err != nil {
					panic("error")
				}
				ports = addPortNumber(ports, portNum)
			}
		}
		if err := scanner.Err(); err != nil {
			panic(err)
		}
	} else {
		portNum, err := getPortNumber(c.LocalSocket)
		if err != nil {
			panic("error")
		}
		ports = addPortNumber(ports, portNum)
	}
	conn_ctx = context.WithValue(conn_ctx, constants.CtxFileTokenType, c.FileTokenType)

	var wg sync.WaitGroup
	log.Infof("Ports count is %d", len(ports))
	var connections sync.Map

	for _, listenPort := range ports {
		localSocket := strings.Split(c.LocalSocket, ":")
		listener, err := net.Listen(strings.ToLower(localSocket[0]), (localSocket[1] + ":" + strconv.Itoa(listenPort)))
		if err != nil {
			panic(err)
		}
		c.ListenerByPort[listenPort] = &listener
		log.Infow("Client endpoint start listen", "listen address", listener.Addr())
		time.Sleep(30 * time.Millisecond)
		wg.Add(1)
		go func(ctx context.Context, listener net.Listener, listenPort int, logger log.Logger) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				conn, err := listener.Accept()
				if err != nil {
					logger.Errorw("Client app connect failed", "error", err.Error())
					continue
				}

				remoteAddr := conn.RemoteAddr().String()
				_, loaded := connections.LoadOrStore(remoteAddr, true)
				if loaded {
					// already connected, close it
					conn.Close()
					continue
				}

				logger := logger.WithValues(constants.ClientAppAddr, remoteAddr)
				logger.Info("Client connection accepted, prepare to establish tunnel with server endpoint for this connection.")

				go func(ctx context.Context, conn net.Conn, logger log.Logger) {
					defer func() {
						conn.Close()
						logger.Info("Tunnel closed")
						connections.Delete(remoteAddr)
					}()

					ctx = context.WithValue(ctx, constants.CtxClientAppAddr, remoteAddr)
					stream, err := session.OpenStreamSync(ctx)
					if err != nil {
						logger.Errorw("Failed to open stream to server endpoint.", "error", err.Error())
						return
					}
					defer stream.Close()
					logger = logger.WithValues(constants.StreamID, stream.StreamID())
					logger.Infow("Stream opened")

					hsh := tunnel.NewHandshakeHelper(constants.TokenLength, handshake)
					hsh.TokenSource = &c.TokenSource

					tun := tunnel.NewTunnel(&stream, constants.ClientEndpoint)
					tun.Conn = &conn
					tun.Hsh = &hsh
					tun.AccessPort = strconv.Itoa(listenPort)

					if !tun.HandShake(ctx) {
						return
					}
					tun.Establish(ctx)
				}(logger.WithContext(ctx), conn, logger)
			}
		}(conn_ctx, listener, listenPort, log.WithValues(constants.ClientAccessPort, listenPort))
	}

	wg.Wait()
}

func getPortNumber(s string) (int, error) {
	port := strings.Split(s, ":")[2]
	return strconv.Atoi(port)
}

// Define a helper function to add a port number to the ports slice.
func addPortNumber(ports []int, portNumber int) []int {
	for _, p := range ports {
		if p == portNumber {
			// The port number is already in the slice, no need to add it again.
			return ports
		}
	}
	// The port number is not in the slice, add it.
	return append(ports, portNumber)
}

func handshake(ctx context.Context, stream *quic.Stream, hsh *tunnel.HandshakeHelper, accessPort string) (bool, *net.Conn) {
	logger := log.FromContext(ctx)
	logger.Info("Starting handshake with server endpoint")
	token := ""
	var err error
	if ctx.Value(constants.CtxFileTokenType) == constants.FileTokenTypePort {
		token, err = (*hsh.TokenSource).GetToken(accessPort)
		if err != nil {
			logger.Errorw("Encounter error.", "erros", err.Error())
			return false, nil
		}
	} else {
		token, err = (*hsh.TokenSource).GetToken(fmt.Sprint(ctx.Value(constants.CtxClientAppAddr)))
		if err != nil {
			logger.Errorw("Encounter error.", "erros", err.Error())
			return false, nil
		}
	}
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

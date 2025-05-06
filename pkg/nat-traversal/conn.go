package nattraversal

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/pion/ice/v2"
	"github.com/pion/sdp/v3"
)

type Connection struct {
	Conn *ice.Conn
	// localSD sdp.SessionDescription
	// remoteTopic string
}

type ConnCtrl struct {
	//	RemoteSD sdp.SessionDescription
	Nt options.NATTraversalOptions
	// Control side transition flag, set to true when the first nat traversal fails
	ControllingConvert bool
	// The mqtt publish hold tag is used to enable subscribers to receive pre-published messages
	MqttRetained    bool
	ExitChan        chan struct{}
	ConvertExitChan chan struct{}
	ConnChan        chan *Connection
	IceAgent        *ice.Agent
	ConvertIceAgent *ice.Agent
	SdCh            chan sdp.SessionDescription
	RemoteSd        sdp.SessionDescription
	MqttClient      MQTTClient
	Connection      *Connection
}

func NewConnCtrl(nt *options.NATTraversalOptions) *ConnCtrl {
	connCtrl := &ConnCtrl{
		Nt:                 *nt,
		ControllingConvert: false,
		MqttRetained:       false,
		ExitChan:           make(chan struct{}),
		ConvertExitChan:    make(chan struct{}),
		ConnChan:           make(chan *Connection),
		SdCh:               make(chan sdp.SessionDescription),
	}
	return connCtrl
}

func ListenUDP(ctx context.Context, connCtrl *ConnCtrl) {

	connection := &Connection{Conn: nil}
	iceAgent := newICEAgent(connCtrl)

	if connCtrl.ControllingConvert {
		connCtrl.ConvertIceAgent = iceAgent
	} else {
		connCtrl.IceAgent = iceAgent
	}

	remoteUfrag, _ := connCtrl.RemoteSd.Attribute("ice-ufrag")
	remotePwd, _ := connCtrl.RemoteSd.Attribute("ice-pwd")
	err := iceAgent.SetRemoteCredentials(remoteUfrag, remotePwd)
	if err != nil {
		panic(err)
	}
	for _, attr := range connCtrl.RemoteSd.Attributes {
		if attr.IsICECandidate() {
			candi, err := ice.UnmarshalCandidate(attr.Value)
			if err != nil {
				panic(err)
			}
			if err := iceAgent.AddRemoteCandidate(candi); err != nil {
				panic(err)
			}
		}
	}

	// Wait for the gather candidates to complete and write the candidates information to localSD
	gaterCompleteChan := make(chan bool)
	localSD, err := sdp.NewJSEPSessionDescription(false)
	if err != nil {
		panic(err)
	}
	_ = iceAgent.OnCandidate(func(c ice.Candidate) {
		if c != nil {
			log.Debugf("Candidate info: %s\n", c.Marshal())
			localSD.WithValueAttribute(sdp.AttrKeyCandidate, c.Marshal())
		} else {
			gaterCompleteChan <- true
		}
	})
	_ = iceAgent.OnConnectionStateChange(func(cs ice.ConnectionState) {
		log.Debugf("OnConnectionStateChange :%s\n", cs.String())
		if connCtrl.Connection == nil {
			if cs == ice.ConnectionStateFailed && connCtrl.ControllingConvert {
				log.Warn("the second nat traversal failed!")
				connCtrl.ConvertExitChan <- struct{}{}
			} else if cs == ice.ConnectionStateFailed && !connCtrl.ControllingConvert {
				log.Warn("the first nat traversal failed!")
				connCtrl.Nt.MQTTTopicKey, _ = connCtrl.RemoteSd.Attribute("convert-topic")
				if connCtrl.Nt.STUNServerURLConvert != "" {
					connCtrl.Nt.STUNServerURL = connCtrl.Nt.STUNServerURLConvert
				}
				connCtrl.ControllingConvert = true
				connCtrl.MqttRetained = true
				connCtrl.IceAgent.Close()
				connCtrl.ExitChan <- struct{}{}
			}
		}
	})
	_ = iceAgent.OnSelectedCandidatePairChange(func(c1, c2 ice.Candidate) {
		log.Debugf("OnSelectedCandidatePairChange c1: %s\n", c1.String())
		log.Debugf("OnSelectedCandidatePairChange c2: %s\n", c2.String())
	})
	if err = iceAgent.GatherCandidates(); err != nil {
		panic(err)
	}
	<-gaterCompleteChan

	// Get the local auth details and save to localSD
	username, password, err := iceAgent.GetLocalUserCredentials()
	if err != nil {
		panic(err)
	}
	localSD.WithValueAttribute("ice-ufrag", username)
	localSD.WithValueAttribute("ice-pwd", password)

	payload, err := localSD.Marshal()
	if err != nil {
		panic(err)
	}
	// Get the remote topic and publish the localSD message
	remoteTopic, _ := connCtrl.RemoteSd.Attribute("remote-topic")
	connCtrl.MqttClient.RemoteTopic = fmt.Sprintf("kungze.com/quic-tun/%s", remoteTopic)
	log.Debugf("Publish msg, topic: %s", connCtrl.MqttClient.RemoteTopic)
	Publish(connCtrl.MqttClient, payload, true)

	log.Debug("iceAgent start accept")
	// Accept blocks until at least one ice candidate pair has successfully connected.
	conn, err := iceAgent.Accept(ctx, remoteUfrag, remotePwd)
	log.Debug("ice agent accept exec")
	if err != nil {
		switch err {
		case ice.ErrCanceledByCaller:
			log.Warn("iceAgent accept canceled by caller")
		case ice.ErrClosed:
			log.Warn("iceAgent accept canceled by closed")
		default:
			panic(err)
		}
	}
	if conn != nil {
		connection.Conn = conn
		connCtrl.Connection = connection
		connCtrl.ConnChan <- connection
	}
}

func DialUDP(ctx context.Context, connCtrl *ConnCtrl) {
	connection := &Connection{Conn: nil}
	iceAgent := newICEAgent(connCtrl)
	if connCtrl.ControllingConvert {
		connCtrl.ConvertIceAgent = iceAgent
	} else {
		connCtrl.IceAgent = iceAgent
	}

	// Wait for the gather candidates to complete and write the candidates information to localSD
	gaterCompleteChan := make(chan struct{})
	localSD, err := sdp.NewJSEPSessionDescription(false)
	if err != nil {
		panic(err)
	}
	// Generate local topic and faild convert topic
	localTopic := uuid.NewString()
	if connCtrl.ControllingConvert {
		localTopic = connCtrl.Nt.MQTTTopicKey + "-" + localTopic
	}
	localSD.WithValueAttribute("remote-topic", localTopic)
	convertTopic := "convert-" + localTopic
	localSD.WithValueAttribute("convert-topic", convertTopic)

	// When we have gathered a new ICE Candidate write it to the localSD
	_ = iceAgent.OnCandidate(func(c ice.Candidate) {
		if c != nil {
			log.Debugf("Candidate info : %s\n", c.Marshal())
			localSD.WithValueAttribute(sdp.AttrKeyCandidate, c.Marshal())
		} else {
			gaterCompleteChan <- struct{}{}
		}
	})
	// When ICE Connection state has change print to stdout
	_ = iceAgent.OnConnectionStateChange(func(cs ice.ConnectionState) {
		log.Debugf("OnConnectionStateChange :%s\n", cs.String())
		if connCtrl.Connection == nil {
			if cs == ice.ConnectionStateFailed && connCtrl.ControllingConvert {
				log.Warn("the second nat traversal failed!")
				connCtrl.ConvertExitChan <- struct{}{}
			} else if cs == ice.ConnectionStateFailed && !connCtrl.ControllingConvert {
				log.Warn("the first nat traversal failed!")
				connCtrl.Nt.MQTTTopicKey = convertTopic
				if connCtrl.Nt.STUNServerURLConvert != "" {
					connCtrl.Nt.STUNServerURL = connCtrl.Nt.STUNServerURLConvert
				}
				connCtrl.ControllingConvert = true
				connCtrl.IceAgent.Close()
				connCtrl.ExitChan <- struct{}{}
			}
		}
	})
	_ = iceAgent.OnSelectedCandidatePairChange(func(c1, c2 ice.Candidate) {
		log.Debugf("OnSelectedCandidatePairChange c1: %s\n", c1.String())
		log.Debugf("OnSelectedCandidatePairChange c2: %s\n", c2.String())
	})
	if err = iceAgent.GatherCandidates(); err != nil {
		panic(err)
	}
	<-gaterCompleteChan

	// Get the local auth details and save to localSD
	username, password, err := iceAgent.GetLocalUserCredentials()
	if err != nil {
		panic(err)
	}
	localSD.WithValueAttribute("ice-ufrag", username)
	localSD.WithValueAttribute("ice-pwd", password)

	// Get the remote topic from the configuration and publish messages to the topic
	mqttClient := NewMQTTClient(connCtrl.Nt, connCtrl.SdCh)
	mqttClient.RemoteTopic = fmt.Sprintf("kungze.com/quic-tun/%s", connCtrl.Nt.MQTTTopicKey)
	payload, err := localSD.Marshal()
	if err != nil {
		panic(err)
	}
	log.Debugf("Publish msg, topic: %s", mqttClient.RemoteTopic)
	Publish(mqttClient, payload, connCtrl.MqttRetained)

	// Set up the mqtt topic and subscribe to it
	mqttClient.Topic = fmt.Sprintf("kungze.com/quic-tun/%s", localTopic)
	Subscribe(mqttClient)
	// Receives and sets the ice agent with this message
	connCtrl.RemoteSd = <-connCtrl.SdCh
	remoteUfrag, _ := connCtrl.RemoteSd.Attribute("ice-ufrag")
	remotePwd, _ := connCtrl.RemoteSd.Attribute("ice-pwd")
	err = iceAgent.SetRemoteCredentials(remoteUfrag, remotePwd)
	if err != nil {
		panic(err)
	}
	for _, attr := range connCtrl.RemoteSd.Attributes {
		if attr.IsICECandidate() {
			candi, err := ice.UnmarshalCandidate(attr.Value)
			if err != nil {
				panic(err)
			}
			if err := iceAgent.AddRemoteCandidate(candi); err != nil {
				panic(err)
			}
		}
	}
	log.Debug("iceAgent start dial")
	// Dial blocks until at least one ice candidate pair has successfully connected.
	conn, err := iceAgent.Dial(ctx, remoteUfrag, remotePwd)
	log.Debug("ice agent dial exec")
	if err != nil {
		switch err {
		case ice.ErrCanceledByCaller:
			log.Warn("ice agent dial canceled by caller")
		case ice.ErrClosed:
			log.Warn("ice agent dial canceled by closed")
		default:
			panic(err)
		}
	}
	if conn != nil {
		connection.Conn = conn
		connCtrl.Connection = connection
		connCtrl.ConnChan <- connection
	}
}

func newICEAgent(connCtrl *ConnCtrl) *ice.Agent {
	iceFailedTimeout := time.Duration(connCtrl.Nt.ICEFailedTimeout) * time.Second
	disconnectedTimeout := 10 * time.Second

	iceAgent, err := ice.NewAgent(
		&ice.AgentConfig{
			Urls:                getICEUrls(connCtrl),
			NetworkTypes:        []ice.NetworkType{ice.NetworkTypeUDP4},
			Lite:                false,
			FailedTimeout:       &iceFailedTimeout,
			DisconnectedTimeout: &disconnectedTimeout,
		})
	if err != nil {
		panic(err)
	}
	return iceAgent
}

func getICEUrls(connCtrl *ConnCtrl) []*ice.URL {
	iceUrls := []*ice.URL{}
	if connCtrl.Nt.STUNServerURL != "" {
		iceSTUNScheme := ice.SchemeTypeSTUN
		if connCtrl.Nt.STUNServerSecure {
			iceSTUNScheme = ice.SchemeTypeSTUNS
		}
		iceSTUNUrl := ice.URL{Scheme: iceSTUNScheme, Host: connCtrl.Nt.STUNServerURL, Port: 3478, Proto: ice.ProtoTypeUDP, Username: connCtrl.Nt.STUNServerUsername, Password: connCtrl.Nt.STUNServerPassword}
		iceUrls = append(iceUrls, &iceSTUNUrl)
	}
	if connCtrl.ControllingConvert {
		if connCtrl.Nt.TURNServerURL != "" {
			iceTURNScheme := ice.SchemeTypeTURN
			if connCtrl.Nt.TURNServerSecure {
				iceTURNScheme = ice.SchemeTypeTURNS
			}
			iceTURNUrl := ice.URL{Scheme: iceTURNScheme, Host: connCtrl.Nt.STUNServerURL, Port: 3478, Proto: ice.ProtoTypeUDP, Username: connCtrl.Nt.STUNServerUsername, Password: connCtrl.Nt.STUNServerPassword}
			iceUrls = append(iceUrls, &iceTURNUrl)
		}
	}
	return iceUrls
}

// ReadFrom reads a packet from the connection,
// copying the payload into p. It returns the number of
// bytes copied into p and the return address that
// was on the packet.
// It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered. Callers should always process
// the n > 0 bytes returned before considering the error err.
// ReadFrom can be made to time out and return an error after a
// fixed time limit; see SetDeadline and SetReadDeadline.
func (c *Connection) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	num, err := c.Conn.Read(p)
	return num, &net.UDPAddr{}, err
}

// WriteTo writes a packet with payload p to addr.
// WriteTo can be made to time out and return an Error after a
// fixed time limit; see SetDeadline and SetWriteDeadline.
// On packet-oriented connections, write timeouts are rare.
func (c *Connection) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Conn.Write(p)
}

// Close closes the connection.
// Any blocked ReadFrom or WriteTo operations will be unblocked and return errors.
func (c *Connection) Close() error {
	return c.Conn.Close()
}

// LocalAddr returns the local network address, if known.
func (c *Connection) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful ReadFrom or WriteTo calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Connection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets the deadline for future ReadFrom calls
// and any currently-blocked ReadFrom call.
// A zero value for t means ReadFrom will not time out.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the deadline for future WriteTo calls
// and any currently-blocked WriteTo call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means WriteTo will not time out.
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return nil
}

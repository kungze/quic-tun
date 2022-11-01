package nattraversal

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/pion/ice/v2"
	"github.com/pion/randutil"
	"github.com/pion/sdp/v3"
)

type connection struct {
	Conn *ice.Conn
	// localSD sdp.SessionDescription
	remoteTopic string
}

type ConnConfig struct {
	//	RemoteSD sdp.SessionDescription
	Nt options.NATTraversalOptions
}

func newSessionID() (uint64, error) {
	// https://tools.ietf.org/html/draft-ietf-rtcweb-jsep-26#section-5.2.1
	// Session ID is recommended to be constructed by generating a 64-bit
	// quantity with the highest bit set to zero and the remaining 63-bits
	// being cryptographically random.
	id, err := randutil.CryptoUint64()
	return id & (^(uint64(1) << 63)), err
}

func ListenUDP(opt *ConnConfig) *connection {
	sdCh := make(chan sdp.SessionDescription)
	mqttClient := NewMQTTClient(opt.Nt, sdCh)
	Subscribe(mqttClient)
	remoteSd := <-sdCh
	var iceScheme ice.SchemeType
	switch strings.ToLower(opt.Nt.ICEServerScheme) {
	case "stun":
		iceScheme = ice.SchemeTypeSTUN
	case "turn":
		iceScheme = ice.SchemeTypeTURN
	case "stuns":
		iceScheme = ice.SchemeTypeSTUNS
	case "turns":
		iceScheme = ice.SchemeTypeTURNS
	default:
		panic(fmt.Errorf("Invalid ICE sheme: %s", opt.Nt.ICEServerScheme))
	}
	iceUrl := ice.URL{Scheme: iceScheme, Host: opt.Nt.ICEServerURL, Port: 3478, Proto: ice.ProtoTypeUDP, Username: opt.Nt.ICEServerUsername, Password: opt.Nt.ICEServerPassword}
	iceAgent, err := ice.NewAgent(
		&ice.AgentConfig{
			Urls:         []*ice.URL{&iceUrl},
			NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
			Lite:         false,
		})
	if err != nil {
		panic(err)
	}

	remoteUfrag, _ := remoteSd.Attribute("ice-ufrag")
	remotePwd, _ := remoteSd.Attribute("ice-pwd")
	err = iceAgent.SetRemoteCredentials(remoteUfrag, remotePwd)
	if err != nil {
		panic(err)
	}
	for _, attr := range remoteSd.Attributes {
		if attr.IsICECandidate() {
			candi, err := ice.UnmarshalCandidate(attr.Value)
			if err != nil {
				panic(err)
			}
			iceAgent.AddRemoteCandidate(candi)
		}
	}

	remoteTopic, _ := remoteSd.Attribute("remote-topic")
	mqttClient.RemoteTopic = fmt.Sprintf("kungze.com/quic-tun/%s", remoteTopic)

	gaterCompleteChan := make(chan bool)
	localSD, err := sdp.NewJSEPSessionDescription(false)
	if err != nil {
		panic(err)
	}
	// var localSD = new(sdp.SessionDescription)
	// sid, err := newSessionID()
	// if err != nil {
	// 	panic(err)
	// }
	// localSD.Origin = sdp.Origin{
	// 	Username:       "kungze",
	// 	SessionID:      sid,
	// 	SessionVersion: uint64(time.Now().Unix()),
	// 	NetworkType:    "IN",
	// 	AddressType:    "IP4",
	// 	UnicastAddress: "0.0.0.0",
	// }
	iceAgent.OnCandidate(func(c ice.Candidate) {
		if c != nil {
			fmt.Printf("%s\n", c.Marshal())
			localSD.WithValueAttribute("candidate", c.Marshal())
			//localMD = localMD.WithCandidate(c.Marshal())
			// localSD.WithValueAttribute(sdp.AttrKeyCandidate, c.Marshal())
		} else {
			gaterCompleteChan <- true
		}
	})

	iceAgent.OnConnectionStateChange(func(cs ice.ConnectionState) {
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxx OnConnectionStateChange :%s\n", cs.String())
	})
	iceAgent.OnSelectedCandidatePairChange(func(c1, c2 ice.Candidate) {
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxxxxx OnSelectedCandidatePairChange c1: %s\n", c1.String())
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxxxxx OnSelectedCandidatePairChange c2: %s\n", c2.String())
	})
	err = iceAgent.GatherCandidates()
	if err != nil {
		panic(err)
	}

	username, password, err := iceAgent.GetLocalUserCredentials()
	if err != nil {
		panic(err)
	}
	localSD.WithValueAttribute("ice-ufrag", username)
	localSD.WithValueAttribute("ice-pwd", password)

	<-gaterCompleteChan
	payload, err := localSD.Marshal()
	if err != nil {
		panic(err)
	}
	Publish(mqttClient, payload)
	user, pwd, err := iceAgent.GetRemoteUserCredentials()
	if err != nil {
		panic(err)
	}
	conn, err := iceAgent.Accept(context.Background(), user, pwd)
	if err != nil {
		panic(err)
	}

	return &connection{Conn: conn}
}

func DialUDP(opt *ConnConfig) *connection {

	var iceScheme ice.SchemeType
	switch strings.ToLower(opt.Nt.ICEServerScheme) {
	case "stun":
		iceScheme = ice.SchemeTypeSTUN
	case "turn":
		iceScheme = ice.SchemeTypeTURN
	case "stuns":
		iceScheme = ice.SchemeTypeSTUNS
	case "turns":
		iceScheme = ice.SchemeTypeTURNS
	default:
		panic(fmt.Errorf("Invalid ICE sheme: %s", opt.Nt.ICEServerScheme))
	}
	iceUrl := ice.URL{Scheme: iceScheme, Host: opt.Nt.ICEServerURL, Port: 3478, Proto: ice.ProtoTypeUDP, Username: opt.Nt.ICEServerUsername, Password: opt.Nt.ICEServerPassword}
	iceAgent, err := ice.NewAgent(
		&ice.AgentConfig{
			Urls:         []*ice.URL{&iceUrl},
			NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
			Lite:         false,
		})
	if err != nil {
		panic(err)
	}

	gaterCompleteChan := make(chan bool)

	localSD, err := sdp.NewJSEPSessionDescription(false)
	if err != nil {
		panic(err)
	}

	// var localSD = new(sdp.SessionDescription)

	// sid, err := newSessionID()
	// if err != nil {
	// 	panic(err)
	// }
	// localSD.Origin = sdp.Origin{
	// 	Username:       "kungze",
	// 	SessionID:      sid,
	// 	SessionVersion: uint64(time.Now().Unix()),
	// 	NetworkType:    "IN",
	// 	AddressType:    "IP4",
	// 	UnicastAddress: "0.0.0.0",
	// }

	iceAgent.OnCandidate(func(c ice.Candidate) {
		if c != nil {
			fmt.Printf("%s\n", c.Marshal())
			localSD.WithValueAttribute("candidate", c.Marshal())
			//localMD = localMD.WithCandidate(c.Marshal())
			// localSD.WithValueAttribute(sdp.AttrKeyCandidate, c.Marshal())
		} else {
			gaterCompleteChan <- true
		}
	})

	iceAgent.OnConnectionStateChange(func(cs ice.ConnectionState) {
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxx OnConnectionStateChange :%s\n", cs.String())
	})
	iceAgent.OnSelectedCandidatePairChange(func(c1, c2 ice.Candidate) {
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxxxxx OnSelectedCandidatePairChange c1: %s\n", c1.String())
		fmt.Printf("xxxxxxxxxxxxxxxxxxxxxxxxxx OnSelectedCandidatePairChange c2: %s\n", c2.String())
	})
	err = iceAgent.GatherCandidates()
	if err != nil {
		panic(err)
	}

	username, password, err := iceAgent.GetLocalUserCredentials()
	if err != nil {
		panic(err)
	}
	//	localTopic := fmt.Sprintf("kungze.com/quic-tun/%s", uuid.NewString())
	localTopic := uuid.NewString()
	localSD.WithValueAttribute("ice-ufrag", username)
	localSD.WithValueAttribute("ice-pwd", password)
	localSD.WithValueAttribute("remote-topic", localTopic)
	<-gaterCompleteChan
	sdCh := make(chan sdp.SessionDescription)
	mqttClient := NewMQTTClient(opt.Nt, sdCh)
	payload, err := localSD.Marshal()
	if err != nil {
		panic(err)
	}
	mqttClient.RemoteTopic = fmt.Sprintf("kungze.com/quic-tun/%s", opt.Nt.MQTTTopicKey)
	Publish(mqttClient, payload)
	// mqttClient.Topic = uuid.NewString()
	mqttClient.Topic = fmt.Sprintf("kungze.com/quic-tun/%s", localTopic)
	Subscribe(mqttClient)
	remoteSD := <-sdCh
	remoteUfrag, _ := remoteSD.Attribute("ice-ufrag")
	remotePwd, _ := remoteSD.Attribute("ice-pwd")
	err = iceAgent.SetRemoteCredentials(remoteUfrag, remotePwd)
	if err != nil {
		panic(err)
	}
	for _, attr := range remoteSD.Attributes {
		if attr.IsICECandidate() {
			candi, err := ice.UnmarshalCandidate(attr.Value)
			if err != nil {
				panic(err)
			}
			iceAgent.AddRemoteCandidate(candi)
		}
	}
	user, pwd, err := iceAgent.GetRemoteUserCredentials()
	if err != nil {
		panic(err)
	}
	fmt.Println("iceAgent xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx001")
	conn, err := iceAgent.Dial(context.Background(), user, pwd)
	if err != nil {
		panic(err)
	}
	return &connection{Conn: conn}
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
func (c *connection) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	num, err := c.Conn.Read(p)
	return num, &net.UDPAddr{}, err
}

// WriteTo writes a packet with payload p to addr.
// WriteTo can be made to time out and return an Error after a
// fixed time limit; see SetDeadline and SetWriteDeadline.
// On packet-oriented connections, write timeouts are rare.
func (c *connection) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Conn.Write(p)
}

// Close closes the connection.
// Any blocked ReadFrom or WriteTo operations will be unblocked and return errors.
func (c *connection) Close() error {
	return c.Conn.Close()
}

// LocalAddr returns the local network address, if known.
func (c *connection) LocalAddr() net.Addr {
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
func (c *connection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets the deadline for future ReadFrom calls
// and any currently-blocked ReadFrom call.
// A zero value for t means ReadFrom will not time out.
func (c *connection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the deadline for future WriteTo calls
// and any currently-blocked WriteTo call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means WriteTo will not time out.
func (c *connection) SetWriteDeadline(t time.Time) error {
	return nil
}

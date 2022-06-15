package tunnel

import (
	"context"
	"net"
	"strings"

	"github.com/kungze/quic-tun/pkg/token"
	"github.com/lucas-clemente/quic-go"
)

type handshakefunc func(context.Context, *quic.Stream, *HandshakeHelper) (bool, *net.Conn)

type HandshakeHelper struct {
	// The function used to process handshake, the handshake processes
	// are different between server endpoint and client endpoint.
	Handshakefunc handshakefunc `json:"-"`
	TokenSource   *token.TokenSourcePlugin
	TokenParser   *token.TokenParserPlugin
	// The data will send to remote endpoint. In client
	// endpoit, this means 'token'; In server endpoint,
	// this means ack message.
	SendData []byte
	// Used to store the data receive from remote endpoint. In
	// server endpoint, this store the 'token'; In client endpoint,
	// this store the ack message.
	ReceiveData string
}

func (h *HandshakeHelper) Write(b []byte) (int, error) {
	h.ReceiveData = strings.ReplaceAll(string(b), "\x00", "")
	return len(b), nil
}

func (h *HandshakeHelper) Read(p []byte) (n int, err error) {
	copy(p, h.SendData)
	return len(h.SendData), nil
}

// Set the data which will be send to remote endpoint.
// For client endpoint, this means set a token; for server endpoint,
// this means set a ack message.
func (h *HandshakeHelper) SetSendData(data []byte) {
	// We wish that different tokens or different ack massages have same
	// length, so we don't use '=' operator to change the value, replace
	// of use 'copy' method.
	copy(h.SendData, data)
}

func NewHandshakeHelper(length int, hsf handshakefunc) HandshakeHelper {
	// Make a fixed length data, we wish that the message's length is
	// explicit and constant in handshake stage.
	data := make([]byte, length)
	return HandshakeHelper{SendData: data, Handshakefunc: hsf}
}

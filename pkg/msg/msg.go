package msg

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	TypeNatHoleQServer = iota + 0x01 // 0x01 quictun-server
	TypeNatHoleQClient               // 0x02 quictun-client
)

const (
	TypeNatHoleResp = iota + 0x80 // 0x81
)

// Frame indicates a protocol message
type FramePayload []byte

type StreamFrameCodec interface {
	Encode(io.Writer, FramePayload) error
	Decode(io.Reader) (FramePayload, error)
}

var ErrShortWrite = errors.New("short write")
var ErrShortRead = errors.New("short read")

type frameCodec struct{}

func NewFrameCodec() StreamFrameCodec {
	return &frameCodec{}
}

func (p *frameCodec) Encode(w io.Writer, framePayload FramePayload) error {
	var f = framePayload
	var totalLen int32 = int32(len(framePayload)) + 4

	err := binary.Write(w, binary.BigEndian, &totalLen)
	if err != nil {
		return err
	}

	n, err := w.Write([]byte(f))
	if err != nil {
		return err
	}

	if n != len(framePayload) {
		return ErrShortWrite
	}
	return nil
}

func (p *frameCodec) Decode(r io.Reader) (FramePayload, error) {
	var totalLen int32
	err := binary.Read(r, binary.BigEndian, &totalLen)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, totalLen-4)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	if n != int(totalLen-4) {
		return nil, ErrShortRead
	}

	return FramePayload(buf), nil
}

type Packet interface {
	Decode([]byte) error     // []byte -> struct
	Encode() ([]byte, error) //  struct -> []byte
}

type NatHoleQServer struct {
	SignKey string
}

func (s *NatHoleQServer) Decode(pktBody []byte) error {
	err := json.Unmarshal(pktBody, s)
	if err != nil {
		return err
	}
	return nil
}

func (s *NatHoleQServer) Encode() ([]byte, error) {
	content, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return content, nil
}

type NatHoleQClient struct {
	SignKey string
}

func (s *NatHoleQClient) Decode(pktBody []byte) error {
	err := json.Unmarshal(pktBody, s)
	if err != nil {
		return err
	}
	return nil
}

func (s *NatHoleQClient) Encode() ([]byte, error) {
	content, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return content, nil
}

type NatHoleResp struct {
	QServerAddr string
	QClientAddr string
}

func (s *NatHoleResp) Decode(pktBody []byte) error {
	err := json.Unmarshal(pktBody, s)
	if err != nil {
		return err
	}
	return nil
}

func (s *NatHoleResp) Encode() ([]byte, error) {
	content, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func Decode(packet []byte) (Packet, error) {
	typeID := packet[0]
	pktBody := packet[1:]

	switch typeID {
	case TypeNatHoleQServer:
		ns := NatHoleQServer{}
		err := ns.Decode(pktBody)
		if err != nil {
			return nil, err
		}
		return &ns, nil
	case TypeNatHoleQClient:
		nc := NatHoleQClient{}
		err := nc.Decode(pktBody)
		if err != nil {
			return nil, err
		}
		return &nc, nil
	case TypeNatHoleResp:
		na := NatHoleResp{}
		err := na.Decode(pktBody)
		if err != nil {
			return nil, err
		}
		return &na, nil
	default:
		return nil, fmt.Errorf("unknown typeID [%d]", typeID)
	}
}

func Encode(p Packet) ([]byte, error) {
	var typeID uint8
	var pktBody []byte
	var err error

	switch t := p.(type) {
	case *NatHoleQServer:
		typeID = TypeNatHoleQServer
		pktBody, err = p.Encode()
		if err != nil {
			return nil, err
		}
	case *NatHoleQClient:
		typeID = TypeNatHoleQClient
		pktBody, err = p.Encode()
		if err != nil {
			return nil, err
		}
	case *NatHoleResp:
		typeID = TypeNatHoleResp
		pktBody, err = p.Encode()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown type [%s]", t)
	}
	return bytes.Join([][]byte{{typeID}, pktBody}, nil), nil
}

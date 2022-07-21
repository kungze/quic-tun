package classifier

const (
	// The max lenght of the traffic header data which the quic-tun will cache them and use them to classify traffic
	HeaderLength = 1024
)

type HeaderCache struct {
	Header []byte
}

func (h *HeaderCache) Write(b []byte) (int, error) {
	if len(h.Header) < HeaderLength {
		h.Header = append(h.Header, b...)
		return len(b), nil
	} else {
		return 0, nil
	}
}

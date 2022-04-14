package handshake

import "strings"

type handshakeHelper struct {
	SendData    []byte
	ReceiveData string
}

func NewHandshakeHelper(sendData []byte, length int) handshakeHelper {
	data := make([]byte, length)
	copy(data, sendData)
	return handshakeHelper{SendData: data}
}

func (h *handshakeHelper) Write(b []byte) (int, error) {
	h.ReceiveData = strings.ReplaceAll(string(b), "\x00", "")
	return len(b), nil
}

func (h *handshakeHelper) Read(p []byte) (n int, err error) {
	copy(p, h.SendData)
	return len(h.SendData), nil
}

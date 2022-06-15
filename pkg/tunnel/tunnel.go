package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/lucas-clemente/quic-go"
	"k8s.io/klog/v2"
)

type tunnel struct {
	Stream             *quic.Stream     `json:"-"`
	Conn               *net.Conn        `json:"-"`
	Hsh                *HandshakeHelper `json:"-"`
	Uuid               uuid.UUID        `json:"uuid"`
	StreamID           quic.StreamID    `json:"streamId"`
	Endpoint           string           `json:"endpoint"`
	ClientAppAddr      string           `json:"clientAppAddr"`
	ServerAppAddr      string           `json:"serverAppAddr"`
	RemoteEndpointAddr string           `json:"remoteEndpointAddr"`
	CreatedAt          string           `json:"createdAt"`
}

func (t *tunnel) HandShake(ctx context.Context) bool {
	res, conn := t.Hsh.Handshakefunc(ctx, t.Stream, t.Hsh)
	if conn != nil {
		t.Conn = conn
	}
	return res
}

func (t *tunnel) Establish(ctx context.Context) {
	logger := klog.FromContext(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	go t.conn2Stream(logger, &wg)
	go t.stream2Conn(logger, &wg)
	logger.Info("Tunnel established successful")
	t.fillProperties(ctx)
	DataStore.Store(t.Uuid, *t)
	wg.Wait()
	DataStore.Delete(t.Uuid)
	logger.Info("Tunnel closed")
}

func (t *tunnel) fillProperties(ctx context.Context) {
	t.StreamID = (*t.Stream).StreamID()
	if t.Endpoint == constants.ClientEndpoint {
		t.ClientAppAddr = (*t.Conn).RemoteAddr().String()
	}
	if t.Endpoint == constants.ServerEndpoint {
		t.ServerAppAddr = (*t.Conn).RemoteAddr().String()
	}
	t.RemoteEndpointAddr = fmt.Sprint(ctx.Value(constants.CtxRemoteEndpointAddr))
	t.CreatedAt = time.Now().String()
}

func (t *tunnel) stream2Conn(logger klog.Logger, wg *sync.WaitGroup) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	_, err := io.Copy(*t.Conn, *t.Stream)
	if err != nil {
		logger.Error(err, "Can not forward packet from QUIC stream to TCP/UNIX socket")
	}
}

func (t *tunnel) conn2Stream(logger klog.Logger, wg *sync.WaitGroup) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	_, err := io.Copy(*t.Stream, *t.Conn)
	if err != nil {
		logger.Error(err, "Can not forward packet from TCP/UNIX socket to QUIC stream")
	}
}

func NewTunnel(stream *quic.Stream, endpoint string) tunnel {
	return tunnel{
		Uuid:     uuid.New(),
		Stream:   stream,
		Endpoint: endpoint,
	}
}

package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kungze/quic-tun/pkg/classifier"
	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/lucas-clemente/quic-go"
)

type tunnel struct {
	Stream             *quic.Stream     `json:"-"`
	Conn               *net.Conn        `json:"-"`
	Hsh                *HandshakeHelper `json:"-"`
	Uuid               uuid.UUID        `json:"uuid"`
	StreamID           quic.StreamID    `json:"streamId"`
	Endpoint           string           `json:"endpoint"`
	ClientAppAddr      string           `json:"clientAppAddr,omitempty"`
	ServerAppAddr      string           `json:"serverAppAddr,omitempty"`
	RemoteEndpointAddr string           `json:"remoteEndpointAddr"`
	CreatedAt          string           `json:"createdAt"`
	SendTotalFlow      int64            `json:"sendTotalFlow"`
	RecvTotalFlow      int64            `json:"recvTotalFlow"`
	Protocol           string           `json:"protocol"`
	ProtocolProperties any              `json:"protocolProperties"`
	// Used to cache the header data from QUIC stream
	streamCache *classifier.HeaderCache
	// Used to cache the header data from TCP/UNIX socket connection
	connCache *classifier.HeaderCache
}

// Before the tunnel establishment, client endpoint and server endpoint need to
// process handshake steps (client endpoint send token, server endpont parse and verify token)
func (t *tunnel) HandShake(ctx context.Context) bool {
	res, conn := t.Hsh.Handshakefunc(ctx, t.Stream, t.Hsh)
	if conn != nil {
		t.Conn = conn
	}
	return res
}

func (t *tunnel) Establish(ctx context.Context) {
	logger := log.FromContext(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	var (
		sendFlowChan = make(chan int64, 1024)
		recvFlowChan = make(chan int64, 1024)
	)
	t.fillProperties(ctx, sendFlowChan, recvFlowChan)
	DataStore.Store(t.Uuid, *t)
	go t.conn2Stream(logger, &wg, sendFlowChan)
	go t.stream2Conn(logger, &wg, recvFlowChan)
	logger.Info("Tunnel established successful")
	// If the tunnel already prepare to close but the analyze
	// process still is running, we need to cancle it by concle context.
	ctx, cancle := context.WithCancel(ctx)
	defer cancle()
	go t.analyze(ctx)
	wg.Wait()
	DataStore.Delete(t.Uuid)
	logger.Info("Tunnel closed")
}

func (t *tunnel) analyze(ctx context.Context) {
	discrs := classifier.LoadDiscriminators()
	var res int
	// We don't know that the number and time the traffic data pass through the tunnel.
	// This means we cannot know what time we can get the enough data in order to we can
	// distinguish the protocol of the traffic that pass through the tunnel. So, we set
	// a time ticker, periodic to analy the header data until has discirminator affirm the
	// traffic or all discirminators deny the traffic.
	timeTick := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			DataStore.Delete(t.Uuid)
			return
		case <-timeTick.C:
			for protocol, discr := range discrs {
				//  In client endpoint, connCache store client application header data, streamCache
				// store server application header data; In server endpoint, them is inverse.
				if t.Endpoint == constants.ClientEndpoint {
					res = discr.AnalyzeHeader(ctx, &t.connCache.Header, &t.streamCache.Header)
				} else {
					res = discr.AnalyzeHeader(ctx, &t.streamCache.Header, &t.connCache.Header)
				}
				// If the discriminator deny the traffic header, we delete it.
				if res == classifier.DENY {
					delete(discrs, protocol)
					t.ProtocolProperties = discr.GetProperties(ctx)
				}
				// Once the traffic's protocol was confirmed, we just need remain this discriminator.
				if res == classifier.AFFIRM || res == classifier.INCOMPLETE {
					t.Protocol = protocol
					t.ProtocolProperties = discr.GetProperties(ctx)
					DataStore.Store(t.Uuid, *t)
					break
				}
			}
			// The protocol was affirmed or all discriminators deny it.
			if res == classifier.AFFIRM || len(discrs) == 0 {
				return
			}
		}
	}
}

func (t *tunnel) fillProperties(ctx context.Context, sendFlowChan chan int64, recvFlowChan chan int64) {
	t.StreamID = (*t.Stream).StreamID()
	if t.Endpoint == constants.ClientEndpoint {
		t.ClientAppAddr = (*t.Conn).RemoteAddr().String()
	}
	if t.Endpoint == constants.ServerEndpoint {
		t.ServerAppAddr = (*t.Conn).RemoteAddr().String()
	}
	t.RemoteEndpointAddr = fmt.Sprint(ctx.Value(constants.CtxRemoteEndpointAddr))
	t.CreatedAt = time.Now().String()
	go func() {
		for {
			select {
			case <-sendFlowChan:
				t.SendTotalFlow = int64(<-sendFlowChan)
			case <-recvFlowChan:
				t.RecvTotalFlow = int64(<-recvFlowChan)
			}
			DataStore.Store(t.Uuid, *t)
		}
	}()
}

func (t *tunnel) stream2Conn(logger log.Logger, wg *sync.WaitGroup, recvFlowChan chan int64) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	// Cache the first 1024 byte datas, quic-tun will use them to analy the traffic's protocol
	_, err := io.CopyN(io.MultiWriter(*t.Conn, t.streamCache), *t.Stream, classifier.HeaderLength)
	if err == nil {
		_, err = copyBuffer(*t.Conn, *t.Stream, nil, recvFlowChan)
	}
	if err != nil {
		logger.Errorw("Can not forward packet from QUIC stream to TCP/UNIX socket", "error", err.Error())
	}
}

func (t *tunnel) conn2Stream(logger log.Logger, wg *sync.WaitGroup, sendFlowChan chan int64) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	// Cache the first 1024 byte datas, quic-tun will use them to analy the traffic's protocol
	_, err := io.CopyN(io.MultiWriter(*t.Stream, t.connCache), *t.Conn, classifier.HeaderLength)
	if err == nil {
		_, err = copyBuffer(*t.Stream, *t.Conn, nil, sendFlowChan)
	}
	if err != nil {
		logger.Errorw("Can not forward packet from TCP/UNIX socket to QUIC stream", "error", err.Error())
	}
}

func NewTunnel(stream *quic.Stream, endpoint string) tunnel {
	var streamCache classifier.HeaderCache
	var connCache classifier.HeaderCache
	streamCache = classifier.HeaderCache{}
	connCache = classifier.HeaderCache{}
	return tunnel{
		Uuid:        uuid.New(),
		Stream:      stream,
		Endpoint:    endpoint,
		streamCache: &streamCache,
		connCache:   &connCache,
	}
}

func copyBuffer(dst io.Writer, src io.Reader, buf []byte, FlowChan chan int64) (written int64, err error) {
	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			FlowChan <- written
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

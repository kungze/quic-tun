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
	ServerTotalBytes   int64            `json:"serverTotalBytes"`
	ClientTotalBytes   int64            `json:"clientTotalBytes"`
	ServerSendRate     string           `json:"serverSendRate"`
	ClientSendRate     string           `json:"clientSendRate"`
	Protocol           string           `json:"protocol"`
	ProtocolProperties any              `json:"protocolProperties"`
	// Used to cache the header data from QUIC stream
	streamCache *classifier.HeaderCache
	// Used to cache the header data from TCP/UNIX socket connection
	connCache  *classifier.HeaderCache
	AccessPort string
}

// Before the tunnel establishment, client endpoint and server endpoint need to
// process handshake steps (client endpoint send token, server endpont parse and verify token)
func (t *tunnel) HandShake(ctx context.Context) bool {
	res, conn := t.Hsh.Handshakefunc(ctx, t.Stream, t.Hsh, t.AccessPort)
	if conn != nil {
		t.Conn = conn
	}
	return res
}

func (t *tunnel) countTraffic(ctx context.Context, stream2conn, conn2stream <-chan int) {
	var s2cTotal, s2cPreTotal, c2sTotal, c2sPreTotal int64
	var s2cRate, c2sRate float64
	var tmp int
	timeTick := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case tmp = <-stream2conn:
			s2cTotal += int64(tmp)
		case tmp = <-conn2stream:
			c2sTotal += int64(tmp)
		case <-timeTick.C:
			s2cRate = float64((s2cTotal - s2cPreTotal)) / 1024.0
			s2cPreTotal = s2cTotal
			c2sRate = float64((c2sTotal - c2sPreTotal)) / 1024.0
			c2sPreTotal = c2sTotal
		}
		if t.Endpoint == constants.ClientEndpoint {
			t.ServerTotalBytes = s2cTotal
			t.ServerSendRate = fmt.Sprintf("%.2f kB/s", s2cRate)
			t.ClientTotalBytes = c2sTotal
			t.ClientSendRate = fmt.Sprintf("%.2f kB/s", c2sRate)
		}
		if t.Endpoint == constants.ServerEndpoint {
			t.ServerTotalBytes = c2sTotal
			t.ServerSendRate = fmt.Sprintf("%.2f kB/s", c2sRate)
			t.ClientTotalBytes = s2cTotal
			t.ClientSendRate = fmt.Sprintf("%.2f kB/s", s2cRate)
		}
		DataStore.Store(t.Uuid, *t)
	}
}

func (t *tunnel) Establish(ctx context.Context) {
	logger := log.FromContext(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	var (
		steam2conn  = make(chan int, 1024)
		conn2stream = make(chan int, 1024)
	)
	t.fillProperties(ctx)
	DataStore.Store(t.Uuid, *t)
	go t.conn2Stream(logger, &wg, conn2stream)
	go t.stream2Conn(logger, &wg, steam2conn)
	logger.Info("Tunnel established successful")
	// If the tunnel already prepare to close but the analyze
	// process still is running, we need to cancle it by concle context.
	ctx, cancle := context.WithCancel(ctx)
	defer cancle()
	go t.countTraffic(ctx, steam2conn, conn2stream)
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

func (t *tunnel) stream2Conn(logger log.Logger, wg *sync.WaitGroup, forwardNumChan chan<- int) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	// Cache the first 1024 byte datas, quic-tun will use them to analy the traffic's protocol
	err := t.copyN(io.MultiWriter(*t.Conn, t.streamCache), *t.Stream, classifier.HeaderLength, forwardNumChan)
	if err == nil {
		err = t.copy(*t.Conn, *t.Stream, forwardNumChan)
	}
	if err != nil {
		logger.Errorw("Can not forward packet from QUIC stream to TCP/UNIX socket", "error", err.Error())
	}
}

func (t *tunnel) conn2Stream(logger log.Logger, wg *sync.WaitGroup, forwardNumChan chan<- int) {
	defer func() {
		(*t.Stream).Close()
		(*t.Conn).Close()
		wg.Done()
	}()
	// Cache the first 1024 byte datas, quic-tun will use them to analy the traffic's protocol
	err := t.copyN(io.MultiWriter(*t.Stream, t.connCache), *t.Conn, classifier.HeaderLength, forwardNumChan)
	if err == nil {
		err = t.copy(*t.Stream, *t.Conn, forwardNumChan)
	}
	if err != nil {
		logger.Errorw("Can not forward packet from TCP/UNIX socket to QUIC stream", "error", err.Error())
	}
}

// Rewrite io.CopyN function https://pkg.go.dev/io#CopyN
func (t *tunnel) copyN(dst io.Writer, src io.Reader, n int64, copyNumChan chan<- int) error {
	return t.copy(dst, io.LimitReader(src, n), copyNumChan)
}

// Rewrite io.Copy function https://pkg.go.dev/io#Copy
func (t *tunnel) copy(dst io.Writer, src io.Reader, nwChan chan<- int) (err error) {
	size := 32 * 1024
	if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
		if l.N < 1 {
			size = 1
		} else {
			size = int(l.N)
		}
	}
	buf := make([]byte, size)
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
			nwChan <- nw
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
	return err
}

func NewTunnel(stream *quic.Stream, endpoint string) tunnel {
	var streamCache = classifier.HeaderCache{}
	var connCache = classifier.HeaderCache{}
	return tunnel{
		Uuid:        uuid.New(),
		Stream:      stream,
		Endpoint:    endpoint,
		streamCache: &streamCache,
		connCache:   &connCache,
	}
}

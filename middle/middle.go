package middle

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/kungze/quic-tun/pkg/constants"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/kungze/quic-tun/pkg/msg"
	"github.com/kungze/quic-tun/pkg/options"
	"github.com/lucas-clemente/quic-go"
)

type MiddleEndpoint struct {
	Address   string
	TlsConfig *tls.Config
	UdpPort   int
	// map[string]chan msg.Packet
	SendChs sync.Map
	// map[string]string
	Addrs sync.Map
	// map[string]struct{}
	DoneChs sync.Map
}

func NewMiddleEndpoint(mo *options.MiddleOptions, tlsCfg *tls.Config) *MiddleEndpoint {
	return &MiddleEndpoint{
		TlsConfig: tlsCfg,
		Address:   mo.ListenOn,
		UdpPort:   mo.BindUdpPort,
	}
}

func (s *MiddleEndpoint) Start() {

	listener, err := quic.ListenAddr(s.Address, s.TlsConfig, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	log.Infow("Middle endpoint start up successful", "listen address", listener.Addr())

	for {
		// Wait client endpoint(quictun-server or quictun-client) connection request.
		session, err := listener.Accept(context.Background())
		if err != nil {
			log.Errorw("Encounter error when accept a connection.", "error", err.Error())
		} else {
			parent_ctx := context.WithValue(context.TODO(), constants.CtxRemoteEndpointAddr, session.RemoteAddr().String())
			logger := log.WithValues(constants.ClientEndpointAddr, session.RemoteAddr().String())
			logger.Info("A new client endpoint connect request accepted.")
			go func() {
				stream, err := session.AcceptStream(context.Background())
				if err != nil {
					logger.Errorw("Cannot accept a new stream.", "error", err.Error())
				}
				logger.Infow("Accept client stream")
				ctx := logger.WithContext(parent_ctx)
				sc := NewStreamControl(stream, s)
				go sc.handleStream(ctx)
			}()
		}
	}
}

type StreamControl struct {
	Stream         quic.Stream
	MiddleEndpoint *MiddleEndpoint
	SendCh         chan msg.Packet
	DoneCh         chan struct{}
}

func NewStreamControl(qs quic.Stream, me *MiddleEndpoint) StreamControl {
	return StreamControl{
		Stream:         qs,
		MiddleEndpoint: me,
		SendCh:         make(chan msg.Packet),
		DoneCh:         make(chan struct{}),
	}
}

func (sc *StreamControl) handleStream(ctx context.Context) {
	log := log.FromContext(ctx)
	defer func() {
		sc.Stream.Close()
		log.Info("closed the stream")
	}()

	frameCodec := msg.NewFrameCodec()
	go func() {
		for {
			select {
			case m := <-sc.SendCh:
				log.Info("send nat hole response")
				respFramePayload, err := msg.Encode(m)
				if err != nil {
					log.Warnf("packet encode error: %s", err.Error())
				}
				err = frameCodec.Encode(sc.Stream, respFramePayload)
				if err != nil {
					log.Warnf("frame encode error: %s", err.Error())
					return
				}
				if err != nil {
					log.Warnf("packet encode error: %s", err.Error())
				}
				close(sc.DoneCh)
			case <-sc.DoneCh:
				return
			}
		}
	}()

	// read from the Stream
	// decode the frame to get the payload
	// the payload is undecoded packet
	framePayload, err := frameCodec.Decode(sc.Stream)
	if err != nil {
		log.Warnf("frame decode error: %s", err.Error())
		return
	}

	err = sc.handlePacket(ctx, framePayload)
	if err != nil {
		log.Warnf("handle packet error: %s", err.Error())
		return
	}

	<-sc.DoneCh
}

func (sc *StreamControl) handlePacket(ctx context.Context, framePayload []byte) (err error) {
	var p msg.Packet
	p, err = msg.Decode(framePayload)
	if err != nil {
		log.Errorf("packet decode error: %s", err.Error())
		return
	}

	switch p := p.(type) {
	case *msg.NatHoleQServer:
		// determine if a quictun-client is already connected
		sc.MiddleEndpoint.Addrs.LoadOrStore(p.SignKey, ctx.Value(constants.CtxRemoteEndpointAddr))
		sendCh, ok := sc.MiddleEndpoint.SendChs.LoadOrStore(p.SignKey, sc.SendCh)
		if !ok { // no client connection
			log.Infow("recv quictun-server message, wait the same sign-key's quictun-client connection", "sign-key", p.SignKey)
		} else { // client is already connected
			log.Infow("recv quictun-server message, and send nathole response to client", "sign-key", p.SignKey)
			clientAddr, _ := sc.MiddleEndpoint.Addrs.Load(p.SignKey)

			// clean map
			sc.MiddleEndpoint.Addrs.Delete(p.SignKey)
			sc.MiddleEndpoint.SendChs.Delete(p.SignKey)
			sc.MiddleEndpoint.DoneChs.Delete(p.SignKey)

			// send natholeResp to client
			sendCh.(chan msg.Packet) <- &msg.NatHoleResp{
				QServerAddr: ctx.Value(constants.CtxRemoteEndpointAddr).(string),
			}
			sc.SendCh <- &msg.NatHoleResp{
				QClientAddr: clientAddr.(string),
			}
		}
	case *msg.NatHoleQClient:
		// determine if a quictun-server is already connected
		sc.MiddleEndpoint.Addrs.LoadOrStore(p.SignKey, ctx.Value(constants.CtxRemoteEndpointAddr))
		sendCh, ok := sc.MiddleEndpoint.SendChs.LoadOrStore(p.SignKey, sc.SendCh)
		if !ok { // no server connection
			log.Infow("recv quictun-client message, wait the same sign-key's quictun-server connection", "sign-key", p.SignKey)
		} else { // server is already connected
			log.Infow("recv quictun-client message, and send nathole response to server", "sign-key", p.SignKey)
			serverAddr, _ := sc.MiddleEndpoint.Addrs.Load(p.SignKey)

			// clean map
			sc.MiddleEndpoint.Addrs.Delete(p.SignKey)
			sc.MiddleEndpoint.SendChs.Delete(p.SignKey)
			sc.MiddleEndpoint.DoneChs.Delete(p.SignKey)

			// send natholeResp to client
			sendCh.(chan msg.Packet) <- &msg.NatHoleResp{
				QClientAddr: ctx.Value(constants.CtxRemoteEndpointAddr).(string),
			}
			sc.SendCh <- &msg.NatHoleResp{
				QServerAddr: serverAddr.(string),
			}
		}
	default:
		return fmt.Errorf("unknown packet type")
	}
	return
}

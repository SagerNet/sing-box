package masque

import (
	std_bufio "bufio"
	"context"
	"errors"
	"io"
	"sync"

	transportHTTP "github.com/sagernet/sing-box/transport/http"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	upgradeToken       = "connect-ip"
	DefaultMTU         = 1280
	PacketHeadroom     = transportHTTP.CapsuleHeadroom
	QUICPacketOverhead = 51
	minimumLinkMTU     = 1280
	maxPacketSize      = 65535
)

type sessionHandler interface {
	handleAddressAssign(addresses []AssignedAddress) error
	handleAddressRequest(addresses []AssignedAddress) error
	handleRouteAdvertisement(routes []AddressRange) error
	handlePacket(buffer *buf.Buffer)
	handlePacketTooBig(buffer *buf.Buffer, mtu int)
}

type session struct {
	ctx            context.Context
	cancel         context.CancelCauseFunc
	stream         io.ReadWriteCloser
	datagrams      transportHTTP.DatagramStream
	reader         *std_bufio.Reader
	handler        sessionHandler
	packetHeadroom func() int
	writeAccess    sync.Mutex
}

func newSession(ctx context.Context, stream io.ReadWriteCloser, handler sessionHandler, packetHeadroom func() int) *session {
	sessionCtx, cancel := context.WithCancelCause(ctx)
	datagrams, _ := stream.(transportHTTP.DatagramStream)
	current := &session{
		ctx:            sessionCtx,
		cancel:         cancel,
		stream:         stream,
		datagrams:      datagrams,
		reader:         std_bufio.NewReader(stream),
		handler:        handler,
		packetHeadroom: packetHeadroom,
	}
	return current
}

func (s *session) run() error {
	stop := context.AfterFunc(s.ctx, func() {
		s.stream.Close()
	})
	defer stop()
	var loops sync.WaitGroup
	if s.datagrams != nil {
		loops.Go(s.loopDatagram)
	}
	err := s.loopCapsule()
	s.cancel(err)
	s.stream.Close()
	loops.Wait()
	return context.Cause(s.ctx)
}

func (s *session) loopDatagram() {
	for {
		datagram, err := s.datagrams.ReceiveDatagram(s.ctx)
		if err != nil {
			s.cancel(err)
			return
		}
		contextID, contextLength, valid := transportHTTP.DecodeVarint(datagram)
		if !valid || contextID != 0 || len(datagram) == contextLength {
			continue
		}
		headroom := s.packetHeadroom()
		buffer := buf.NewSize(headroom + len(datagram) - contextLength)
		buffer.Resize(headroom, 0)
		common.Must1(buffer.Write(datagram[contextLength:]))
		s.handler.handlePacket(buffer)
	}
}

func (s *session) loopCapsule() error {
	for {
		capsuleType, _, err := transportHTTP.ReadVarint(s.reader)
		if err != nil {
			return err
		}
		length, _, err := transportHTTP.ReadVarint(s.reader)
		if err != nil {
			return err
		}
		if length > transportHTTP.MaxCapsuleLength {
			return E.New("capsule too large: ", length)
		}
		switch capsuleType {
		case transportHTTP.CapsuleTypeDatagram:
			err = s.readDatagramCapsule(int(length))
		case capsuleTypeAddressAssign, capsuleTypeAddressRequest, capsuleTypeRouteAdvertisement:
			err = s.readControlCapsule(capsuleType, int(length))
		default:
			_, err = s.reader.Discard(int(length))
		}
		if err != nil {
			return err
		}
	}
}

func (s *session) readDatagramCapsule(length int) error {
	contextID, contextLength, err := transportHTTP.ReadVarint(s.reader)
	if err != nil {
		return err
	}
	if contextLength > length {
		return E.New("malformed datagram capsule")
	}
	payloadLength := length - contextLength
	if contextID != 0 || payloadLength == 0 || payloadLength > maxPacketSize {
		_, err = s.reader.Discard(payloadLength)
		return err
	}
	headroom := s.packetHeadroom()
	buffer := buf.NewSize(headroom + payloadLength)
	buffer.Resize(headroom, 0)
	_, err = buffer.ReadFullFrom(s.reader, payloadLength)
	if err != nil {
		buffer.Release()
		return err
	}
	s.handler.handlePacket(buffer)
	return nil
}

func (s *session) readControlCapsule(capsuleType uint64, length int) error {
	payload := make([]byte, length)
	_, err := io.ReadFull(s.reader, payload)
	if err != nil {
		return err
	}
	switch capsuleType {
	case capsuleTypeAddressAssign:
		addresses, parseErr := parseAddresses(payload)
		if parseErr != nil {
			return E.Cause(parseErr, "parse ADDRESS_ASSIGN capsule")
		}
		return s.handler.handleAddressAssign(addresses)
	case capsuleTypeAddressRequest:
		addresses, parseErr := parseAddresses(payload)
		if parseErr != nil {
			return E.Cause(parseErr, "parse ADDRESS_REQUEST capsule")
		}
		if len(addresses) == 0 {
			return E.New("empty ADDRESS_REQUEST capsule")
		}
		for _, address := range addresses {
			if address.RequestID == 0 {
				return E.New("ADDRESS_REQUEST capsule with zero request ID")
			}
		}
		return s.handler.handleAddressRequest(addresses)
	default:
		routes, parseErr := parseRoutes(payload)
		if parseErr != nil {
			return E.Cause(parseErr, "parse ROUTE_ADVERTISEMENT capsule")
		}
		return s.handler.handleRouteAdvertisement(routes)
	}
}

func (s *session) writeCapsule(capsule *buf.Buffer) error {
	defer capsule.Release()
	s.writeAccess.Lock()
	defer s.writeAccess.Unlock()
	_, err := s.stream.Write(capsule.Bytes())
	return err
}

func (s *session) writePackets(buffers []*buf.Buffer) error {
	capsules := buffers[:0]
	for i, buffer := range buffers {
		datagram := transportHTTP.PrependContextID(buffer)
		if s.datagrams != nil {
			err := s.datagrams.SendDatagram(datagram.Bytes())
			var tooLarge *transportHTTP.DatagramTooLargeError
			switch {
			case err == nil:
				datagram.Release()
				continue
			case errors.As(err, &tooLarge):
				mtu := tooLarge.MaxPayloadSize - 1
				if mtu >= minimumLinkMTU {
					datagram.Advance(1)
					s.handler.handlePacketTooBig(datagram, mtu)
					continue
				}
				err = E.New("QUIC connection is unable to carry ", minimumLinkMTU, " bytes packets")
			case errors.Is(err, transportHTTP.ErrDatagramUnsupported):
				capsules = append(capsules, datagram)
				continue
			}
			datagram.Release()
			buf.ReleaseMulti(capsules)
			buf.ReleaseMulti(buffers[i+1:])
			return err
		}
		capsules = append(capsules, datagram)
	}
	if len(capsules) == 0 {
		return nil
	}
	s.writeAccess.Lock()
	defer s.writeAccess.Unlock()
	return transportHTTP.WriteDatagramCapsules(s.stream, capsules)
}

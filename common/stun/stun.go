package stun

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	DefaultServer = "stun.voipgate.com:3478"

	magicCookie = 0x2112A442
	headerSize  = 20

	bindingRequest         = 0x0001
	bindingSuccessResponse = 0x0101
	bindingErrorResponse   = 0x0111

	attrMappedAddress    = 0x0001
	attrChangeRequest    = 0x0003
	attrErrorCode        = 0x0009
	attrXORMappedAddress = 0x0020
	attrOtherAddress     = 0x802c

	familyIPv4 = 0x01
	familyIPv6 = 0x02

	changeIP   = 0x04
	changePort = 0x02

	defaultRTO    = 500 * time.Millisecond
	minRTO        = 250 * time.Millisecond
	maxRetransmit = 2
)

type Phase int32

const (
	PhaseBinding Phase = iota
	PhaseNATMapping
	PhaseNATFiltering
	PhaseDone
)

type NATMapping int32

const (
	NATMappingUnknown NATMapping = iota
	_                            // reserved
	NATMappingEndpointIndependent
	NATMappingAddressDependent
	NATMappingAddressAndPortDependent
)

func (m NATMapping) String() string {
	switch m {
	case NATMappingEndpointIndependent:
		return "Endpoint Independent"
	case NATMappingAddressDependent:
		return "Address Dependent"
	case NATMappingAddressAndPortDependent:
		return "Address and Port Dependent"
	default:
		return "Unknown"
	}
}

type NATFiltering int32

const (
	NATFilteringUnknown NATFiltering = iota
	NATFilteringEndpointIndependent
	NATFilteringAddressDependent
	NATFilteringAddressAndPortDependent
)

func (f NATFiltering) String() string {
	switch f {
	case NATFilteringEndpointIndependent:
		return "Endpoint Independent"
	case NATFilteringAddressDependent:
		return "Address Dependent"
	case NATFilteringAddressAndPortDependent:
		return "Address and Port Dependent"
	default:
		return "Unknown"
	}
}

type TransactionID [12]byte

type Options struct {
	Server     string
	Dialer     N.Dialer
	Context    context.Context
	OnProgress func(Progress)
}

type Progress struct {
	Phase        Phase
	ExternalAddr string
	LatencyMs    int32
	NATMapping   NATMapping
	NATFiltering NATFiltering
}

type Result struct {
	ExternalAddr     string
	LatencyMs        int32
	NATMapping       NATMapping
	NATFiltering     NATFiltering
	NATTypeSupported bool
}

type parsedResponse struct {
	xorMappedAddr netip.AddrPort
	mappedAddr    netip.AddrPort
	otherAddr     netip.AddrPort
}

func (r *parsedResponse) externalAddr() (netip.AddrPort, bool) {
	if r.xorMappedAddr.IsValid() {
		return r.xorMappedAddr, true
	}
	if r.mappedAddr.IsValid() {
		return r.mappedAddr, true
	}
	return netip.AddrPort{}, false
}

type stunAttribute struct {
	typ   uint16
	value []byte
}

func newTransactionID() TransactionID {
	var id TransactionID
	_, _ = rand.Read(id[:])
	return id
}

func buildBindingRequest(txID TransactionID, attrs ...stunAttribute) []byte {
	attrLen := 0
	for _, attr := range attrs {
		attrLen += 4 + len(attr.value) + paddingLen(len(attr.value))
	}

	buf := make([]byte, headerSize+attrLen)
	binary.BigEndian.PutUint16(buf[0:2], bindingRequest)
	binary.BigEndian.PutUint16(buf[2:4], uint16(attrLen))
	binary.BigEndian.PutUint32(buf[4:8], magicCookie)
	copy(buf[8:20], txID[:])

	offset := headerSize
	for _, attr := range attrs {
		binary.BigEndian.PutUint16(buf[offset:offset+2], attr.typ)
		binary.BigEndian.PutUint16(buf[offset+2:offset+4], uint16(len(attr.value)))
		copy(buf[offset+4:offset+4+len(attr.value)], attr.value)
		offset += 4 + len(attr.value) + paddingLen(len(attr.value))
	}

	return buf
}

func changeRequestAttr(flags byte) stunAttribute {
	return stunAttribute{
		typ:   attrChangeRequest,
		value: []byte{0, 0, 0, flags},
	}
}

func parseResponse(data []byte, expectedTxID TransactionID) (*parsedResponse, error) {
	if len(data) < headerSize {
		return nil, E.New("response too short")
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	if msgType&0xC000 != 0 {
		return nil, E.New("invalid STUN message: top 2 bits not zero")
	}

	cookie := binary.BigEndian.Uint32(data[4:8])
	if cookie != magicCookie {
		return nil, E.New("invalid magic cookie")
	}

	var txID TransactionID
	copy(txID[:], data[8:20])
	if txID != expectedTxID {
		return nil, E.New("transaction ID mismatch")
	}

	msgLen := int(binary.BigEndian.Uint16(data[2:4]))
	if msgLen > len(data)-headerSize {
		return nil, E.New("message length exceeds data")
	}

	attrData := data[headerSize : headerSize+msgLen]

	if msgType == bindingErrorResponse {
		return nil, parseErrorResponse(attrData)
	}
	if msgType != bindingSuccessResponse {
		return nil, E.New("unexpected message type: ", fmt.Sprintf("0x%04x", msgType))
	}

	resp := &parsedResponse{}
	offset := 0
	for offset+4 <= len(attrData) {
		attrType := binary.BigEndian.Uint16(attrData[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(attrData[offset+2 : offset+4]))
		if offset+4+attrLen > len(attrData) {
			break
		}
		attrValue := attrData[offset+4 : offset+4+attrLen]

		switch attrType {
		case attrXORMappedAddress:
			addr, err := parseXORMappedAddress(attrValue, txID)
			if err == nil {
				resp.xorMappedAddr = addr
			}
		case attrMappedAddress:
			addr, err := parseMappedAddress(attrValue)
			if err == nil {
				resp.mappedAddr = addr
			}
		case attrOtherAddress:
			addr, err := parseMappedAddress(attrValue)
			if err == nil {
				resp.otherAddr = addr
			}
		}

		offset += 4 + attrLen + paddingLen(attrLen)
	}

	return resp, nil
}

func parseErrorResponse(data []byte) error {
	offset := 0
	for offset+4 <= len(data) {
		attrType := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if offset+4+attrLen > len(data) {
			break
		}
		if attrType == attrErrorCode && attrLen >= 4 {
			attrValue := data[offset+4 : offset+4+attrLen]
			class := int(attrValue[2] & 0x07)
			number := int(attrValue[3])
			code := class*100 + number
			if attrLen > 4 {
				return E.New("STUN error ", code, ": ", string(attrValue[4:]))
			}
			return E.New("STUN error ", code)
		}
		offset += 4 + attrLen + paddingLen(attrLen)
	}
	return E.New("STUN error response")
}

func parseXORMappedAddress(data []byte, txID TransactionID) (netip.AddrPort, error) {
	if len(data) < 4 {
		return netip.AddrPort{}, E.New("XOR-MAPPED-ADDRESS too short")
	}

	family := data[1]
	xPort := binary.BigEndian.Uint16(data[2:4])
	port := xPort ^ uint16(magicCookie>>16)

	switch family {
	case familyIPv4:
		if len(data) < 8 {
			return netip.AddrPort{}, E.New("XOR-MAPPED-ADDRESS IPv4 too short")
		}
		var ip [4]byte
		binary.BigEndian.PutUint32(ip[:], binary.BigEndian.Uint32(data[4:8])^magicCookie)
		return netip.AddrPortFrom(netip.AddrFrom4(ip), port), nil
	case familyIPv6:
		if len(data) < 20 {
			return netip.AddrPort{}, E.New("XOR-MAPPED-ADDRESS IPv6 too short")
		}
		var ip [16]byte
		var xorKey [16]byte
		binary.BigEndian.PutUint32(xorKey[0:4], magicCookie)
		copy(xorKey[4:16], txID[:])
		for i := range 16 {
			ip[i] = data[4+i] ^ xorKey[i]
		}
		return netip.AddrPortFrom(netip.AddrFrom16(ip), port), nil
	default:
		return netip.AddrPort{}, E.New("unknown address family: ", family)
	}
}

func parseMappedAddress(data []byte) (netip.AddrPort, error) {
	if len(data) < 4 {
		return netip.AddrPort{}, E.New("MAPPED-ADDRESS too short")
	}

	family := data[1]
	port := binary.BigEndian.Uint16(data[2:4])

	switch family {
	case familyIPv4:
		if len(data) < 8 {
			return netip.AddrPort{}, E.New("MAPPED-ADDRESS IPv4 too short")
		}
		return netip.AddrPortFrom(
			netip.AddrFrom4([4]byte{data[4], data[5], data[6], data[7]}), port,
		), nil
	case familyIPv6:
		if len(data) < 20 {
			return netip.AddrPort{}, E.New("MAPPED-ADDRESS IPv6 too short")
		}
		var ip [16]byte
		copy(ip[:], data[4:20])
		return netip.AddrPortFrom(netip.AddrFrom16(ip), port), nil
	default:
		return netip.AddrPort{}, E.New("unknown address family: ", family)
	}
}

func roundTrip(conn net.PacketConn, addr net.Addr, txID TransactionID, attrs []stunAttribute, rto time.Duration) (*parsedResponse, time.Duration, error) {
	request := buildBindingRequest(txID, attrs...)
	currentRTO := rto
	retransmitCount := 0

	sendTime := time.Now()
	_, err := conn.WriteTo(request, addr)
	if err != nil {
		return nil, 0, E.Cause(err, "send STUN request")
	}

	buf := make([]byte, 1024)
	for {
		err = conn.SetReadDeadline(sendTime.Add(currentRTO))
		if err != nil {
			return nil, 0, E.Cause(err, "set read deadline")
		}

		n, _, readErr := conn.ReadFrom(buf)
		if readErr != nil {
			if E.IsTimeout(readErr) && retransmitCount < maxRetransmit {
				retransmitCount++
				currentRTO *= 2
				sendTime = time.Now()
				_, err = conn.WriteTo(request, addr)
				if err != nil {
					return nil, 0, E.Cause(err, "retransmit STUN request")
				}
				continue
			}
			return nil, 0, E.Cause(readErr, "read STUN response")
		}

		if n < headerSize || buf[0]&0xC0 != 0 ||
			binary.BigEndian.Uint32(buf[4:8]) != magicCookie {
			continue
		}
		var receivedTxID TransactionID
		copy(receivedTxID[:], buf[8:20])
		if receivedTxID != txID {
			continue
		}

		latency := time.Since(sendTime)

		resp, parseErr := parseResponse(buf[:n], txID)
		if parseErr != nil {
			return nil, 0, parseErr
		}

		return resp, latency, nil
	}
}

func Run(options Options) (*Result, error) {
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	server := options.Server
	if server == "" {
		server = DefaultServer
	}
	serverSocksaddr := M.ParseSocksaddr(server)
	if serverSocksaddr.Port == 0 {
		serverSocksaddr.Port = 3478
	}

	reportProgress := options.OnProgress
	if reportProgress == nil {
		reportProgress = func(Progress) {}
	}

	var (
		packetConn net.PacketConn
		serverAddr net.Addr
		err        error
	)

	if options.Dialer != nil {
		packetConn, err = options.Dialer.ListenPacket(ctx, serverSocksaddr)
		if err != nil {
			return nil, E.Cause(err, "create UDP socket")
		}
		serverAddr = serverSocksaddr
	} else {
		serverUDPAddr, resolveErr := net.ResolveUDPAddr("udp", serverSocksaddr.String())
		if resolveErr != nil {
			return nil, E.Cause(resolveErr, "resolve STUN server")
		}
		packetConn, err = net.ListenPacket("udp", "")
		if err != nil {
			return nil, E.Cause(err, "create UDP socket")
		}
		serverAddr = serverUDPAddr
	}
	defer func() {
		_ = packetConn.Close()
	}()
	if deadline.NeedAdditionalReadDeadline(packetConn) {
		packetConn = deadline.NewPacketConn(bufio.NewPacketConn(packetConn))
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	rto := defaultRTO

	// Phase 1: Binding
	reportProgress(Progress{Phase: PhaseBinding})

	txID := newTransactionID()
	resp, latency, err := roundTrip(packetConn, serverAddr, txID, nil, rto)
	if err != nil {
		return nil, E.Cause(err, "binding request")
	}

	rto = max(minRTO, 3*latency)

	externalAddr, ok := resp.externalAddr()
	if !ok {
		return nil, E.New("no mapped address in response")
	}

	result := &Result{
		ExternalAddr: externalAddr.String(),
		LatencyMs:    int32(latency.Milliseconds()),
	}

	reportProgress(Progress{
		Phase:        PhaseBinding,
		ExternalAddr: result.ExternalAddr,
		LatencyMs:    result.LatencyMs,
	})

	otherAddr := resp.otherAddr
	if !otherAddr.IsValid() {
		result.NATTypeSupported = false
		reportProgress(Progress{
			Phase:        PhaseDone,
			ExternalAddr: result.ExternalAddr,
			LatencyMs:    result.LatencyMs,
		})
		return result, nil
	}
	result.NATTypeSupported = true

	select {
	case <-ctx.Done():
		return result, nil
	default:
	}

	// Phase 2: NAT Mapping Detection (RFC 5780 Section 4.3)
	reportProgress(Progress{
		Phase:        PhaseNATMapping,
		ExternalAddr: result.ExternalAddr,
		LatencyMs:    result.LatencyMs,
	})

	result.NATMapping = detectNATMapping(
		packetConn, serverSocksaddr.Port, externalAddr, otherAddr, rto,
	)

	reportProgress(Progress{
		Phase:        PhaseNATMapping,
		ExternalAddr: result.ExternalAddr,
		LatencyMs:    result.LatencyMs,
		NATMapping:   result.NATMapping,
	})

	select {
	case <-ctx.Done():
		return result, nil
	default:
	}

	// Phase 3: NAT Filtering Detection (RFC 5780 Section 4.4)
	reportProgress(Progress{
		Phase:        PhaseNATFiltering,
		ExternalAddr: result.ExternalAddr,
		LatencyMs:    result.LatencyMs,
		NATMapping:   result.NATMapping,
	})

	result.NATFiltering = detectNATFiltering(packetConn, serverAddr, rto)

	reportProgress(Progress{
		Phase:        PhaseDone,
		ExternalAddr: result.ExternalAddr,
		LatencyMs:    result.LatencyMs,
		NATMapping:   result.NATMapping,
		NATFiltering: result.NATFiltering,
	})

	return result, nil
}

func detectNATMapping(
	conn net.PacketConn,
	serverPort uint16,
	externalAddr netip.AddrPort,
	otherAddr netip.AddrPort,
	rto time.Duration,
) NATMapping {
	// Mapping Test II: Send to other_ip:server_port
	testIIAddr := net.UDPAddrFromAddrPort(
		netip.AddrPortFrom(otherAddr.Addr(), serverPort),
	)
	txID2 := newTransactionID()
	resp2, _, err := roundTrip(conn, testIIAddr, txID2, nil, rto)
	if err != nil {
		return NATMappingUnknown
	}

	externalAddr2, ok := resp2.externalAddr()
	if !ok {
		return NATMappingUnknown
	}

	if externalAddr == externalAddr2 {
		return NATMappingEndpointIndependent
	}

	// Mapping Test III: Send to other_ip:other_port
	testIIIAddr := net.UDPAddrFromAddrPort(otherAddr)
	txID3 := newTransactionID()
	resp3, _, err := roundTrip(conn, testIIIAddr, txID3, nil, rto)
	if err != nil {
		return NATMappingUnknown
	}

	externalAddr3, ok := resp3.externalAddr()
	if !ok {
		return NATMappingUnknown
	}

	if externalAddr2 == externalAddr3 {
		return NATMappingAddressDependent
	}
	return NATMappingAddressAndPortDependent
}

func detectNATFiltering(
	conn net.PacketConn,
	serverAddr net.Addr,
	rto time.Duration,
) NATFiltering {
	// Filtering Test II: Request response from different IP and port
	txID := newTransactionID()
	_, _, err := roundTrip(conn, serverAddr, txID,
		[]stunAttribute{changeRequestAttr(changeIP | changePort)}, rto)
	if err == nil {
		return NATFilteringEndpointIndependent
	}

	// Filtering Test III: Request response from different port only
	txID = newTransactionID()
	_, _, err = roundTrip(conn, serverAddr, txID,
		[]stunAttribute{changeRequestAttr(changePort)}, rto)
	if err == nil {
		return NATFilteringAddressDependent
	}

	return NATFilteringAddressAndPortDependent
}

func paddingLen(n int) int {
	if n%4 == 0 {
		return 0
	}
	return 4 - n%4
}

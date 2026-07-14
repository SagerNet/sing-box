//go:build windows && (amd64 || 386)

package bridge

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/windivert"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun/gtcpip"
	"github.com/sagernet/sing-tun/gtcpip/header"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/sys/windows"
)

const (
	bridgeReservedPortCount uint16 = 1024

	bridgeICMPFlowTimeout = time.Minute

	bridgeDivertPriority int16 = 0

	bridgeDivertRetryDelayMin = 100 * time.Millisecond
	bridgeDivertRetryDelayMax = 2 * time.Second

	bridgeBatchBufferSize = 256 * 1024
)

type divertKind uint8

const (
	divertTransport divertKind = iota
	divertICMPEcho
	divertICMPError
)

type localSegment struct {
	prefix  netip.Prefix
	address netip.Addr
}

type egressState struct {
	inet4         netip.Addr
	inet6         netip.Addr
	mtu           uint32
	inet4Segments []localSegment
	inet6Segments []localSegment
}

func (s *egressState) equal(other *egressState) bool {
	return s.inet4 == other.inet4 && s.inet6 == other.inet6 && s.mtu == other.mtu &&
		slices.Equal(s.inet4Segments, other.inet4Segments) &&
		slices.Equal(s.inet6Segments, other.inet6Segments)
}

// sourceAddress picks the translated source for an outbound packet. Windows
// routes with the strong host model: the route lookup is constrained to the
// interface owning the source address, so choosing the source is what steers
// the packet. Destinations in a connected subnet take that subnet's own
// address; everything else takes the egress address.
func (s *egressState) sourceAddress(destination netip.Addr, isV6 bool) netip.Addr {
	segments := s.inet4Segments
	egressAddress := s.inet4
	if isV6 {
		segments = s.inet6Segments
		egressAddress = s.inet6
	}
	for _, segment := range segments {
		if segment.prefix.Contains(destination) {
			return segment.address
		}
	}
	return egressAddress
}

func (s *egressState) divertAddresses(isV6 bool) []netip.Addr {
	egressAddress := s.inet4
	segments := s.inet4Segments
	if isV6 {
		egressAddress = s.inet6
		segments = s.inet6Segments
	}
	if !egressAddress.IsValid() {
		return nil
	}
	addresses := []netip.Addr{egressAddress}
	for _, segment := range segments {
		if !slices.Contains(addresses, segment.address) {
			addresses = append(addresses, segment.address)
		}
	}
	return addresses
}

type diverter struct {
	handle *windivert.Handle
	done   chan struct{}
}

type backendWindows struct {
	backendBase

	writeAccess  sync.Mutex
	injectHandle *windivert.Handle
	sendBuffer   []byte
	sendAddrs    []windivert.Address

	deliverAccess   sync.Mutex
	deliverBuffer   []byte
	deliverBuffered [][]byte

	egress atomic.Pointer[egressState]

	reservation   *portReservation
	reservedStart uint16

	icmp4, icmp6 *icmpTable

	diverters []*diverter
}

func newBackend(ctx context.Context, logger logger.ContextLogger, networkManager adapter.NetworkManager, tag string, options option.BridgeOutboundOptions) (Backend, error) {
	instance := &backendWindows{}
	err := instance.init(ctx, logger, networkManager, tag, options)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (b *backendWindows) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := b.start()
	if err != nil {
		b.Close()
		return err
	}
	return nil
}

func (b *backendWindows) start() error {
	b.closed = make(chan struct{})

	state := b.currentEgressState()
	b.egress.Store(state)

	err := b.acquireReservations()
	if err != nil {
		return err
	}

	injectHandle, err := windivert.Open(nil, windivert.LayerNetwork, windivert.PriorityHighest, windivert.FlagSendOnly)
	if err != nil {
		return E.Cause(err, "bridge: open injection handle (Administrator required)")
	}
	b.injectHandle = injectHandle
	b.sendBuffer = make([]byte, 0, bridgeBatchBufferSize)
	b.sendAddrs = make([]windivert.Address, 0, windivert.BatchMax)

	b.egressAccess.Lock()
	err = b.rebuildDivertersLocked(state)
	b.egressAccess.Unlock()
	if err != nil {
		return err
	}

	b.registerMonitors(b.syncEgress)
	b.syncEgress()
	state = b.egress.Load()
	if !state.inet4.IsValid() && !state.inet6.IsValid() {
		b.logger.Debug("bridge egress unavailable, dropping forwarded traffic")
	}
	b.logger.Info("bridge started (WinDivert, egress ", b.egressLabel(), ")")
	return nil
}

func (b *backendWindows) egressLabel() string {
	if b.boundInterface != "" {
		return b.boundInterface
	}
	return "auto"
}

func (b *backendWindows) acquireReservations() error {
	reservation, err := acquirePortReservation(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP, bridgeReservedPortCount)
	if err != nil {
		return E.Cause(err, "bridge: reserve ports")
	}
	b.reservation = reservation
	b.reservedStart = reservation.startPort
	b.icmp4 = newICMPTable(bridgeICMPFlowTimeout)
	b.icmp6 = newICMPTable(bridgeICMPFlowTimeout)
	return nil
}

func (b *backendWindows) PortSelectorRange() (uint16, uint16) {
	return b.reservedStart, bridgeReservedPortCount
}

func (b *backendWindows) rebuildDivertersLocked(state *egressState) error {
	b.closeDivertersLocked()

	if b.inet4Port.IsValid() && state.inet4.IsValid() {
		err := b.openFamilyDiverters(state.divertAddresses(false), false)
		if err != nil {
			b.closeDivertersLocked()
			return err
		}
	}
	if b.inet6Port.IsValid() && state.inet6.IsValid() {
		err := b.openFamilyDiverters(state.divertAddresses(true), true)
		if err != nil {
			b.closeDivertersLocked()
			return err
		}
	}
	return nil
}

func (b *backendWindows) closeDivertersLocked() {
	for _, existing := range b.diverters {
		existing.handle.Close()
		<-existing.done
	}
	b.diverters = nil
}

func (b *backendWindows) openFamilyDiverters(addresses []netip.Addr, isV6 bool) error {
	portHigh := uint16(uint32(b.reservedStart) + uint32(bridgeReservedPortCount) - 1)
	entries := []struct {
		what  string
		kind  divertKind
		build func() (*windivert.Filter, error)
	}{
		{"TCP", divertTransport, func() (*windivert.Filter, error) {
			return windivert.InboundTCPPortRange(addresses, b.reservedStart, portHigh)
		}},
		{"UDP", divertTransport, func() (*windivert.Filter, error) {
			return windivert.InboundUDPPortRange(addresses, b.reservedStart, portHigh)
		}},
		{"ICMP echo", divertICMPEcho, func() (*windivert.Filter, error) {
			return windivert.InboundICMPEchoReply(addresses)
		}},
		{"ICMP error", divertICMPError, func() (*windivert.Filter, error) {
			return windivert.InboundICMPError(addresses)
		}},
	}
	for _, entry := range entries {
		filter, err := entry.build()
		if err != nil {
			return E.Cause(err, "bridge: build ", entry.what, " divert filter")
		}
		err = b.openDiverter(filter, entry.kind, isV6)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *backendWindows) openDiverter(filter *windivert.Filter, kind divertKind, isV6 bool) error {
	handle, err := windivert.Open(filter, windivert.LayerNetwork, bridgeDivertPriority, 0)
	if err != nil {
		return E.Cause(err, "bridge: open divert handle")
	}
	d := &diverter{handle: handle, done: make(chan struct{})}
	b.diverters = append(b.diverters, d)
	go b.divertLoop(d, kind, isV6)
	return nil
}

func (b *backendWindows) divertLoop(d *diverter, kind divertKind, isV6 bool) {
	defer close(d.done)
	buffer := make([]byte, bridgeBatchBufferSize)
	deliverBatch := make([][]byte, 0, windivert.BatchMax)
	retryDelay := bridgeDivertRetryDelayMin
	for {
		n, addrs, err := d.handle.RecvBatch(buffer)
		if err != nil {
			if errors.Is(err, windows.ERROR_OPERATION_ABORTED) || errors.Is(err, windows.ERROR_NO_DATA) || errors.Is(err, windows.ERROR_INVALID_HANDLE) {
				return
			}
			select {
			case <-b.closed:
				return
			default:
			}
			b.logger.Debug(E.Cause(err, "bridge divert recv"))
			select {
			case <-b.closed:
				return
			case <-time.After(retryDelay):
			}
			retryDelay = min(retryDelay*2, bridgeDivertRetryDelayMax)
			continue
		}
		retryDelay = bridgeDivertRetryDelayMin
		deliverBatch = deliverBatch[:0]
		offset := 0
		for i := range addrs {
			packetLength := ipPacketLength(buffer[offset:n])
			if packetLength <= 0 || offset+packetLength > n {
				break
			}
			packet := buffer[offset : offset+packetLength]
			offset += packetLength
			if b.classifyInbound(packet, kind, isV6) {
				deliverBatch = append(deliverBatch, packet)
			} else {
				b.reinject(d.handle, packet, &addrs[i])
			}
		}
		if len(deliverBatch) > 0 {
			b.deliver(deliverBatch)
		}
	}
}

func ipPacketLength(packet []byte) int {
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		if len(packet) < header.IPv4MinimumSize {
			return 0
		}
		return int(header.IPv4(packet).TotalLength())
	case header.IPv6Version:
		if len(packet) < header.IPv6MinimumSize {
			return 0
		}
		return header.IPv6MinimumSize + int(header.IPv6(packet).PayloadLength())
	default:
		return 0
	}
}

func (b *backendWindows) classifyInbound(packet []byte, kind divertKind, isV6 bool) bool {
	portAddress := b.inet4Port
	if isV6 {
		portAddress = b.inet6Port
	}
	if !portAddress.IsValid() {
		return false
	}
	switch kind {
	case divertTransport:
		return rewriteAddress(packet, portAddress, false)
	case divertICMPEcho:
		table := b.icmpFor(isV6)
		if table == nil {
			return false
		}
		info, valid := parseTransport(packet, isV6)
		if !valid || info.transport == nil {
			return false
		}
		identifier, identifierValid := icmpIdentifier(info.transport, isV6)
		if !identifierValid || !table.isActive(identifier, packetRemoteAddress(packet, isV6, true)) {
			return false
		}
		return rewriteAddress(packet, portAddress, false)
	case divertICMPError:
		return b.classifyICMPError(packet, portAddress, isV6)
	default:
		return false
	}
}

// classifyICMPError claims an inbound ICMP error whose embedded packet is
// one of our translated outbound packets, and prepares it for the
// dispatcher: only the embedded source address is rewritten back to the
// port address; the dispatcher's ICMP error return path matches the flow
// by the embedded tuple, rewrites everything else, and recomputes the
// outer checksums.
func (b *backendWindows) classifyICMPError(packet []byte, portAddress netip.Addr, isV6 bool) bool {
	info, valid := parseTransport(packet, isV6)
	if !valid || info.transport == nil || info.fragmented {
		return false
	}
	var inner []byte
	if isV6 {
		if len(info.transport) < header.ICMPv6ErrorHeaderSize {
			return false
		}
		if !header.ICMPv6(info.transport).Type().IsErrorType() {
			return false
		}
		inner = info.transport[header.ICMPv6ErrorHeaderSize:]
	} else {
		if len(info.transport) < header.ICMPv4MinimumSize {
			return false
		}
		switch header.ICMPv4(info.transport).Type() {
		case header.ICMPv4DstUnreachable, header.ICMPv4SrcQuench, header.ICMPv4Redirect, header.ICMPv4TimeExceeded, header.ICMPv4ParamProblem:
		default:
			return false
		}
		inner = info.transport[header.ICMPv4MinimumSize:]
	}
	if isV6 {
		return b.rewriteICMPErrorInner6(packet, inner, portAddress)
	}
	return b.rewriteICMPErrorInner4(packet, inner, portAddress)
}

func (b *backendWindows) rewriteICMPErrorInner4(packet, inner []byte, portAddress netip.Addr) bool {
	if len(inner) < header.IPv4MinimumSize {
		return false
	}
	innerHdr := header.IPv4(inner)
	headerLength := int(innerHdr.HeaderLength())
	if headerLength < header.IPv4MinimumSize || headerLength > len(inner) {
		return false
	}
	outerDestination := header.IPv4(packet).DestinationAddr()
	innerSource := innerHdr.SourceAddr()
	if innerSource != outerDestination {
		return false
	}
	transport := inner[headerLength:]
	if !b.embeddedFlowActive(innerHdr.TransportProtocol(), transport, innerHdr.DestinationAddr(), false) {
		return false
	}
	oldAddress := innerSource.As4()
	newAddress := portAddress.As4()
	innerHdr.SetSourceAddressWithChecksumUpdate(tcpip.AddrFrom4(newAddress))
	adjustTransportChecksum(innerHdr.TransportProtocol(), transport, oldAddress[:], newAddress[:])
	return true
}

func (b *backendWindows) rewriteICMPErrorInner6(packet, inner []byte, portAddress netip.Addr) bool {
	if len(inner) < header.IPv6MinimumSize {
		return false
	}
	innerHdr := header.IPv6(inner)
	outerDestination := header.IPv6(packet).DestinationAddr()
	innerSource := innerHdr.SourceAddr()
	if innerSource != outerDestination {
		return false
	}
	transport := inner[header.IPv6MinimumSize:]
	if !b.embeddedFlowActive(innerHdr.TransportProtocol(), transport, innerHdr.DestinationAddr(), true) {
		return false
	}
	oldAddress := innerSource.As16()
	newAddress := portAddress.As16()
	innerHdr.SetSourceAddress(tcpip.AddrFrom16(newAddress))
	adjustTransportChecksum(innerHdr.TransportProtocol(), transport, oldAddress[:], newAddress[:])
	return true
}

func (b *backendWindows) embeddedFlowActive(protocol tcpip.TransportProtocolNumber, transport []byte, remote netip.Addr, isV6 bool) bool {
	switch protocol {
	case header.TCPProtocolNumber, header.UDPProtocolNumber:
		if len(transport) < 4 {
			return false
		}
		return b.portReserved(binary.BigEndian.Uint16(transport[0:2]))
	case header.ICMPv4ProtocolNumber:
		if isV6 || len(transport) < header.ICMPv4MinimumSize {
			return false
		}
		icmpHdr := header.ICMPv4(transport)
		if icmpHdr.Type() != header.ICMPv4Echo {
			return false
		}
		table := b.icmpFor(false)
		return table != nil && table.isActive(icmpHdr.Ident(), remote)
	case header.ICMPv6ProtocolNumber:
		if !isV6 || len(transport) < header.ICMPv6MinimumSize {
			return false
		}
		icmpHdr := header.ICMPv6(transport)
		if icmpHdr.Type() != header.ICMPv6EchoRequest {
			return false
		}
		table := b.icmpFor(true)
		return table != nil && table.isActive(icmpHdr.Ident(), remote)
	default:
		return false
	}
}

func (b *backendWindows) portReserved(port uint16) bool {
	return port >= b.reservedStart && uint32(port) < uint32(b.reservedStart)+uint32(bridgeReservedPortCount)
}

func (b *backendWindows) deliver(packets [][]byte) {
	b.deliverAccess.Lock()
	defer b.deliverAccess.Unlock()

	b.returnAccess.Lock()
	returnPaths := b.returnPaths
	b.returnAccess.Unlock()
	if len(returnPaths) == 0 {
		return
	}
	headroom := returnPaths[0].ReturnHeadroom()

	// The return-path writeback copies synchronously, so the staging buffer is
	// safe to reuse on the next batch.
	total := 0
	for _, packet := range packets {
		total += headroom + len(packet)
	}
	if cap(b.deliverBuffer) < total {
		b.deliverBuffer = make([]byte, total)
	}
	staging := b.deliverBuffer[:total]
	buffered := b.deliverBuffered[:0]
	offset := 0
	for _, packet := range packets {
		segment := staging[offset : offset+headroom+len(packet)]
		copy(segment[headroom:], packet)
		buffered = append(buffered, segment)
		offset += headroom + len(packet)
	}
	b.deliverBuffered = buffered

	unconsumed := buffered
	currentHeadroom := headroom
	for _, returnPath := range returnPaths {
		if len(unconsumed) == 0 {
			break
		}
		nextHeadroom := returnPath.ReturnHeadroom()
		if nextHeadroom != currentHeadroom {
			rebuffered := make([][]byte, 0, len(unconsumed))
			for _, packet := range unconsumed {
				payload := packet[currentHeadroom:]
				buffer := make([]byte, nextHeadroom+len(payload))
				copy(buffer[nextHeadroom:], payload)
				rebuffered = append(rebuffered, buffer)
			}
			unconsumed = rebuffered
			currentHeadroom = nextHeadroom
		}
		unconsumed = returnPath.ReturnPackets(unconsumed)
	}
}

func (b *backendWindows) reinject(handle *windivert.Handle, packet []byte, addr *windivert.Address) {
	_, err := handle.Send(packet, addr)
	if err != nil {
		select {
		case <-b.closed:
		default:
			b.logger.Debug(E.Cause(err, "bridge reinject"))
		}
	}
}

func (b *backendWindows) icmpFor(isV6 bool) *icmpTable {
	if isV6 {
		return b.icmp6
	}
	return b.icmp4
}

func (b *backendWindows) PortMTU() uint32 {
	state := b.egress.Load()
	if state == nil {
		return 0
	}
	return state.mtu
}

func (b *backendWindows) WritePackets(packets [][]byte) error {
	state := b.egress.Load()
	if state == nil {
		return nil
	}
	b.writeAccess.Lock()
	defer b.writeAccess.Unlock()
	for _, packet := range packets {
		if len(packet) == 0 || len(packet) > maxPacketLength {
			continue
		}
		if !b.prepareOutbound(packet, state) {
			continue
		}
		if len(b.sendAddrs) == windivert.BatchMax || len(b.sendBuffer)+len(packet) > cap(b.sendBuffer) {
			b.flushOutboundLocked()
		}
		b.sendBuffer = append(b.sendBuffer, packet...)
		var addr windivert.Address
		addr.SetOutbound(true)
		addr.SetIPv6(header.IPVersion(packet) == header.IPv6Version)
		addr.SetIPChecksum(true)
		addr.SetTCPChecksum(true)
		addr.SetUDPChecksum(true)
		b.sendAddrs = append(b.sendAddrs, addr)
	}
	b.flushOutboundLocked()
	return nil
}

func (b *backendWindows) flushOutboundLocked() {
	if len(b.sendAddrs) == 0 {
		return
	}
	_, err := b.injectHandle.SendBatch(b.sendBuffer, b.sendAddrs)
	if err != nil {
		select {
		case <-b.closed:
		default:
			b.logger.Debug(E.Cause(err, "bridge inject"))
		}
	}
	b.sendBuffer = b.sendBuffer[:0]
	b.sendAddrs = b.sendAddrs[:0]
}

func (b *backendWindows) prepareOutbound(packet []byte, state *egressState) bool {
	var isV6 bool
	switch header.IPVersion(packet) {
	case header.IPv4Version:
	case header.IPv6Version:
		isV6 = true
	default:
		return false
	}
	// The batched injection ioctl walks the buffer by IP total length; a
	// packet with trailing bytes would desynchronize the walk and fail the
	// whole batch.
	if ipPacketLength(packet) != len(packet) {
		return false
	}
	egressAddr := state.sourceAddress(packetRemoteAddress(packet, isV6, false), isV6)
	if !egressAddr.IsValid() {
		return false
	}
	info, valid := parseTransport(packet, isV6)
	if !valid {
		return false
	}
	switch info.protocol {
	case header.TCPProtocolNumber, header.UDPProtocolNumber:
		if info.transport != nil {
			if len(info.transport) < 4 {
				return false
			}
			if !b.portReserved(binary.BigEndian.Uint16(info.transport[0:2])) {
				b.logger.Debug("bridge: dropping outbound packet with source port outside the reserved block")
				return false
			}
		}
	case header.ICMPv4ProtocolNumber, header.ICMPv6ProtocolNumber:
		table := b.icmpFor(isV6)
		if table == nil {
			return false
		}
		if info.transport != nil {
			identifier, identifierValid := icmpIdentifier(info.transport, isV6)
			if !identifierValid {
				return false
			}
			table.register(identifier, packetRemoteAddress(packet, isV6, false))
		}
	default:
		return false
	}
	return rewriteAddressWithInfo(packet, info, egressAddr, true)
}

func (b *backendWindows) syncEgress() {
	b.egressAccess.Lock()
	defer b.egressAccess.Unlock()
	select {
	case <-b.closed:
		return
	default:
	}
	state := b.currentEgressState()
	previous := b.egress.Load()
	if previous != nil &&
		slices.Equal(previous.divertAddresses(false), state.divertAddresses(false)) &&
		slices.Equal(previous.divertAddresses(true), state.divertAddresses(true)) {
		if !previous.equal(state) {
			b.egress.Store(state)
		}
		return
	}
	if (b.inet4Port.IsValid() && !state.inet4.IsValid()) || (b.inet6Port.IsValid() && !state.inet6.IsValid()) {
		b.logger.Debug("bridge egress address unavailable, dropping affected traffic")
	}
	err := b.rebuildDivertersLocked(state)
	if err != nil {
		b.egress.Store(&egressState{})
		b.logger.Debug(E.Cause(err, "bridge rebuild diverters"))
		return
	}
	b.egress.Store(state)
	b.logger.Debug("bridge egress ", b.egressLabel(), " updated")
}

func (b *backendWindows) currentEgressState() *egressState {
	state := &egressState{}
	egressName := b.resolveEgress()
	if egressName == "" {
		return state
	}
	finder := b.networkManager.InterfaceFinder()
	if finder == nil {
		return state
	}
	err := finder.Update()
	if err != nil {
		b.logger.Debug(E.Cause(err, "bridge update interfaces"))
		return state
	}
	egressInterface, err := finder.ByName(egressName)
	if err != nil {
		return state
	}
	if egressInterface.MTU > 0 {
		state.mtu = uint32(egressInterface.MTU)
	}
	for _, prefix := range egressInterface.Addresses {
		address := prefix.Addr().Unmap()
		if address.Is4() {
			if !state.inet4.IsValid() && address.IsGlobalUnicast() {
				state.inet4 = address
			}
		} else if !state.inet6.IsValid() && address.IsGlobalUnicast() {
			state.inet6 = address
		}
	}
	b.collectLocalSegments(state, finder)
	return state
}

// collectLocalSegments gathers the connected subnets whose destinations must
// bypass the egress pin so they leave on their own interface, mirroring the
// Linux backend (routing table) and the Darwin backend (pf pass-in rules).
// With a pinned egress only its own subnets are considered.
func (b *backendWindows) collectLocalSegments(state *egressState, finder control.InterfaceFinder) {
	for _, localInterface := range finder.Interfaces() {
		if b.boundInterface != "" && localInterface.Name != b.boundInterface {
			continue
		}
		if localInterface.Flags&net.FlagUp == 0 || localInterface.Flags&net.FlagBroadcast == 0 ||
			localInterface.Flags&net.FlagLoopback != 0 || localInterface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		for _, prefix := range localInterface.Addresses {
			address := prefix.Addr().Unmap()
			if !address.IsGlobalUnicast() {
				continue
			}
			segment := localSegment{
				prefix:  netip.PrefixFrom(address, prefix.Bits()).Masked(),
				address: address,
			}
			if address.Is4() {
				if state.inet4.IsValid() {
					state.inet4Segments = appendSegment(state.inet4Segments, segment)
				}
			} else if state.inet6.IsValid() {
				state.inet6Segments = appendSegment(state.inet6Segments, segment)
			}
		}
	}
	sortSegments(state.inet4Segments)
	sortSegments(state.inet6Segments)
}

func appendSegment(segments []localSegment, segment localSegment) []localSegment {
	for _, existing := range segments {
		if existing.prefix == segment.prefix {
			return segments
		}
	}
	return append(segments, segment)
}

func sortSegments(segments []localSegment) {
	slices.SortFunc(segments, func(a, b localSegment) int {
		if a.prefix.Bits() != b.prefix.Bits() {
			return b.prefix.Bits() - a.prefix.Bits()
		}
		if result := a.prefix.Addr().Compare(b.prefix.Addr()); result != 0 {
			return result
		}
		return a.address.Compare(b.address)
	})
}

func (b *backendWindows) Close() error {
	b.closeOnce.Do(func() {
		if b.closed != nil {
			close(b.closed)
		}
		if b.unregister != nil {
			b.unregister()
		}
		b.egressAccess.Lock()
		b.closeDivertersLocked()
		b.egressAccess.Unlock()
		if b.injectHandle != nil {
			b.injectHandle.Close()
		}
		b.reservation.Close()
		b.reservation = nil
		releaseBridgeIndex(b.index)
	})
	return nil
}

type transportInfo struct {
	protocol   tcpip.TransportProtocolNumber
	transport  []byte
	fragmented bool
}

func parseTransport(packet []byte, isV6 bool) (transportInfo, bool) {
	if !isV6 {
		if len(packet) < header.IPv4MinimumSize {
			return transportInfo{}, false
		}
		ipHdr := header.IPv4(packet)
		if !ipHdr.IsValid(len(packet)) {
			return transportInfo{}, false
		}
		info := transportInfo{
			protocol:   ipHdr.TransportProtocol(),
			fragmented: ipHdr.More() || ipHdr.FragmentOffset() != 0,
		}
		if ipHdr.FragmentOffset() == 0 {
			info.transport = ipHdr.Payload()
		}
		return info, true
	}
	if len(packet) < header.IPv6MinimumSize {
		return transportInfo{}, false
	}
	ipHdr := header.IPv6(packet)
	payloadLength := int(ipHdr.PayloadLength())
	if payloadLength > len(packet)-header.IPv6MinimumSize {
		return transportInfo{}, false
	}
	payload := packet[header.IPv6MinimumSize:][:payloadLength]
	var info transportInfo
	nextHeader := ipHdr.NextHeader()
	offset := 0
	for {
		switch header.IPv6ExtensionHeaderIdentifier(nextHeader) {
		case header.IPv6HopByHopOptionsExtHdrIdentifier, header.IPv6RoutingExtHdrIdentifier, header.IPv6DestinationOptionsExtHdrIdentifier:
			if len(payload)-offset < 2 {
				return transportInfo{}, false
			}
			extensionLength := (int(payload[offset+1]) + 1) * 8
			if len(payload)-offset < extensionLength {
				return transportInfo{}, false
			}
			nextHeader = payload[offset]
			offset += extensionLength
		case header.IPv6FragmentExtHdrIdentifier:
			if len(payload)-offset < header.IPv6FragmentHeaderSize {
				return transportInfo{}, false
			}
			fragmentHdr := header.IPv6Fragment(payload[offset : offset+header.IPv6FragmentHeaderSize])
			info.fragmented = true
			if fragmentHdr.FragmentOffset() != 0 {
				info.protocol = fragmentHdr.TransportProtocol()
				return info, true
			}
			nextHeader = fragmentHdr.NextHeader()
			offset += header.IPv6FragmentHeaderSize
		case header.IPv6NoNextHeaderIdentifier:
			return transportInfo{}, false
		default:
			info.protocol = tcpip.TransportProtocolNumber(nextHeader)
			info.transport = payload[offset:]
			return info, true
		}
	}
}

func packetRemoteAddress(packet []byte, isV6, inbound bool) netip.Addr {
	if isV6 {
		ipHdr := header.IPv6(packet)
		if inbound {
			return ipHdr.SourceAddr()
		}
		return ipHdr.DestinationAddr()
	}
	ipHdr := header.IPv4(packet)
	if inbound {
		return ipHdr.SourceAddr()
	}
	return ipHdr.DestinationAddr()
}

func icmpIdentifier(transport []byte, isV6 bool) (uint16, bool) {
	if isV6 {
		if len(transport) < header.ICMPv6MinimumSize {
			return 0, false
		}
		return header.ICMPv6(transport).Ident(), true
	}
	if len(transport) < header.ICMPv4MinimumSize {
		return 0, false
	}
	return header.ICMPv4(transport).Ident(), true
}

func rewriteAddress(packet []byte, address netip.Addr, source bool) bool {
	info, valid := parseTransport(packet, header.IPVersion(packet) == header.IPv6Version)
	if !valid {
		return false
	}
	return rewriteAddressWithInfo(packet, info, address, source)
}

func rewriteAddressWithInfo(packet []byte, info transportInfo, address netip.Addr, source bool) bool {
	switch header.IPVersion(packet) {
	case header.IPv4Version:
		ipHdr := header.IPv4(packet)
		newAddress := address.As4()
		var oldAddress [4]byte
		if source {
			copy(oldAddress[:], ipHdr.SourceAddressSlice())
		} else {
			copy(oldAddress[:], ipHdr.DestinationAddressSlice())
		}
		if info.transport != nil {
			adjustTransportChecksum(info.protocol, info.transport, oldAddress[:], newAddress[:])
		}
		if source {
			ipHdr.SetSourceAddressWithChecksumUpdate(tcpip.AddrFrom4(newAddress))
		} else {
			ipHdr.SetDestinationAddressWithChecksumUpdate(tcpip.AddrFrom4(newAddress))
		}
		return true
	case header.IPv6Version:
		ipHdr := header.IPv6(packet)
		newAddress := address.As16()
		var oldAddress [16]byte
		if source {
			copy(oldAddress[:], ipHdr.SourceAddressSlice())
			ipHdr.SetSourceAddress(tcpip.AddrFrom16(newAddress))
		} else {
			copy(oldAddress[:], ipHdr.DestinationAddressSlice())
			ipHdr.SetDestinationAddress(tcpip.AddrFrom16(newAddress))
		}
		if info.transport != nil {
			adjustTransportChecksum(info.protocol, info.transport, oldAddress[:], newAddress[:])
		}
		return true
	default:
		return false
	}
}

func adjustTransportChecksum(protocol tcpip.TransportProtocolNumber, transport []byte, oldData, newData []byte) {
	oldAddress := tcpip.AddrFromSlice(oldData)
	newAddress := tcpip.AddrFromSlice(newData)
	switch protocol {
	case header.TCPProtocolNumber:
		if len(transport) < header.TCPMinimumSize {
			return
		}
		header.TCP(transport).UpdateChecksumPseudoHeaderAddress(oldAddress, newAddress, true)
	case header.UDPProtocolNumber:
		if len(transport) < header.UDPMinimumSize {
			return
		}
		udpHdr := header.UDP(transport)
		if udpHdr.Checksum() == 0 {
			return
		}
		udpHdr.UpdateChecksumPseudoHeaderAddress(oldAddress, newAddress, true)
		if udpHdr.Checksum() == 0 {
			udpHdr.SetChecksum(0xffff)
		}
	case header.ICMPv6ProtocolNumber:
		if len(transport) < header.ICMPv6MinimumSize {
			return
		}
		header.ICMPv6(transport).UpdateChecksumPseudoHeaderAddress(oldAddress, newAddress)
	}
}

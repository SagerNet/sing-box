package route

import (
	"context"
	"io"
	"net"
	"net/netip"
	"slices"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type spliceTarget struct {
	socket        tun.SpliceSocket
	offload       N.PacketOffload
	readCounters  []N.CountFunc
	writeCounters []N.CountFunc
}

func unwrapSpliceTarget(conn any, allowOffload bool) (spliceTarget, bool) {
	var target spliceTarget
	for {
		readCounter, isReadCounter := conn.(N.ReadCounter)
		writeCounter, isWriteCounter := conn.(N.WriteCounter)
		if isReadCounter || isWriteCounter {
			if !isReadCounter || !isWriteCounter {
				return spliceTarget{}, false
			}
			reader, readCounters := readCounter.UnwrapReader()
			writer, writeCounters := writeCounter.UnwrapWriter()
			if any(reader) != any(writer) {
				return spliceTarget{}, false
			}
			target.readCounters = append(target.readCounters, readCounters...)
			target.writeCounters = append(target.writeCounters, writeCounters...)
			conn = reader
			continue
		}
		packetReadCounter, isPacketReadCounter := conn.(N.PacketReadCounter)
		packetWriteCounter, isPacketWriteCounter := conn.(N.PacketWriteCounter)
		if isPacketReadCounter || isPacketWriteCounter {
			if !isPacketReadCounter || !isPacketWriteCounter {
				return spliceTarget{}, false
			}
			reader, readCounters := packetReadCounter.UnwrapPacketReader()
			writer, writeCounters := packetWriteCounter.UnwrapPacketWriter()
			if any(reader) != any(writer) {
				return spliceTarget{}, false
			}
			target.readCounters = append(target.readCounters, readCounters...)
			target.writeCounters = append(target.writeCounters, writeCounters...)
			conn = reader
			continue
		}
		if allowOffload {
			upstream, offload := N.UnwrapPacketOffload(conn)
			if offload != nil {
				socket, isSocket := upstream.(tun.SpliceSocket)
				if !isSocket {
					return spliceTarget{}, false
				}
				target.socket = socket
				target.offload = offload
				return target, true
			}
		}
		readerWithUpstream, isReaderWithUpstream := conn.(N.ReaderWithUpstream)
		if !isReaderWithUpstream || !readerWithUpstream.ReaderReplaceable() {
			return spliceTarget{}, false
		}
		writerWithUpstream, isWriterWithUpstream := conn.(N.WriterWithUpstream)
		if !isWriterWithUpstream || !writerWithUpstream.WriterReplaceable() {
			return spliceTarget{}, false
		}
		socket, isSocket := conn.(tun.SpliceSocket)
		if isSocket {
			target.socket = socket
			return target, true
		}
		withUpstream, hasUpstream := conn.(common.WithUpstream)
		if hasUpstream {
			conn = withUpstream.Upstream()
			continue
		}
		upstreamReader, hasUpstreamReader := conn.(N.WithUpstreamReader)
		upstreamWriter, hasUpstreamWriter := conn.(N.WithUpstreamWriter)
		if !hasUpstreamReader || !hasUpstreamWriter {
			return spliceTarget{}, false
		}
		reader := upstreamReader.UpstreamReader()
		if reader != upstreamWriter.UpstreamWriter() {
			return spliceTarget{}, false
		}
		conn = reader
	}
}

func (m *ConnectionManager) spliceClose(ctx context.Context, conn io.Closer, remote io.Closer, onClose N.CloseHandlerFunc) N.CloseHandlerFunc {
	return func(err error) {
		if err == nil {
			m.logger.DebugContext(ctx, "connection finished")
		} else if !E.IsClosedOrCanceled(err) {
			m.logger.ErrorContext(ctx, "connection closed: ", err)
		} else {
			m.logger.TraceContext(ctx, "connection closed")
		}
		if onClose != nil {
			onClose(err)
		}
		common.Close(conn, remote)
	}
}

func (m *ConnectionManager) spliceConnection(ctx context.Context, conn net.Conn, remoteConn net.Conn, onClose N.CloseHandlerFunc) (bool, error) {
	destination, writeCounters := N.UnwrapCountWriter(conn, nil)
	goConn, isGoConn := N.CastWriter[*tun.GoConn](destination)
	if !isGoConn {
		return false, nil
	}
	target, isTarget := unwrapSpliceTarget(remoteConn, false)
	if !isTarget {
		return false, nil
	}
	var (
		source       io.Reader = conn
		readCounters []N.CountFunc
	)
	for {
		source, readCounters = N.UnwrapCountReader(source, readCounters)
		cachedReader, isCached := source.(N.CachedReader)
		if !isCached {
			break
		}
		buffer := cachedReader.ReadCached()
		if buffer == nil {
			break
		}
		dataLen := buffer.Len()
		_, err := remoteConn.Write(buffer.Bytes())
		buffer.Release()
		if err != nil {
			return false, err
		}
		for _, counter := range readCounters {
			counter(int64(dataLen))
		}
	}
	goReader, isGoReader := N.CastReader[*tun.GoConn](source)
	if !isGoReader || goReader != goConn {
		return false, nil
	}
	return goConn.Splice(target.socket, tun.SpliceOptions{
		ReadCounters:  append(readCounters, target.writeCounters...),
		WriteCounters: append(target.readCounters, writeCounters...),
		OnClose:       m.spliceClose(ctx, conn, remoteConn, onClose),
	}), nil
}

type spliceSource struct {
	natConn       *tun.UDPNatConn
	cachedReaders []N.CachedPacketReader
	readCounters  []N.CountFunc
	writeCounters []N.CountFunc
}

func unwrapSpliceSource(conn N.PacketConn) (spliceSource, bool) {
	var source spliceSource
	writer, writeCounters := N.UnwrapCountPacketWriter(conn, nil)
	natWriter, isNATWriter := N.CastPacketWriter[*tun.UDPNatConn](writer)
	if !isNATWriter {
		return spliceSource{}, false
	}
	source.writeCounters = writeCounters
	var reader N.PacketReader = conn
	for {
		readCounter, isReadCounter := reader.(N.PacketReadCounter)
		if isReadCounter {
			upstreamReader, readCounters := readCounter.UnwrapPacketReader()
			source.readCounters = append(source.readCounters, readCounters...)
			reader = upstreamReader
			continue
		}
		natReader, isNATReader := reader.(*tun.UDPNatConn)
		if isNATReader {
			if natReader != natWriter {
				return spliceSource{}, false
			}
			source.natConn = natReader
			return source, true
		}
		cachedReader, isCached := reader.(N.CachedPacketReader)
		if isCached {
			source.cachedReaders = append(source.cachedReaders, cachedReader)
		} else {
			readerWithUpstream, isReaderWithUpstream := reader.(N.ReaderWithUpstream)
			if !isReaderWithUpstream || !readerWithUpstream.ReaderReplaceable() {
				return spliceSource{}, false
			}
		}
		withUpstream, hasUpstream := reader.(common.WithUpstream)
		if hasUpstream {
			reader, _ = withUpstream.Upstream().(N.PacketReader)
		} else {
			upstreamReader, hasUpstreamReader := reader.(N.WithUpstreamReader)
			if !hasUpstreamReader {
				return spliceSource{}, false
			}
			reader, _ = upstreamReader.UpstreamReader().(N.PacketReader)
		}
		if reader == nil {
			return spliceSource{}, false
		}
	}
}

func (s *spliceSource) takeCached() []*N.PacketBuffer {
	var cached []*N.PacketBuffer
	for _, cachedReader := range s.cachedReaders {
		packet := cachedReader.ReadCachedPacket()
		if packet == nil {
			continue
		}
		if packet.Buffer == nil {
			N.PutPacketBuffer(packet)
			continue
		}
		cached = append(cached, packet)
	}
	return cached
}

func (m *ConnectionManager) splicePacketConnection(ctx context.Context, conn N.PacketConn, remote any, metadata *adapter.InboundContext, destinationAddress netip.Addr, udpTimeout time.Duration, onClose N.CloseHandlerFunc) (N.PacketConn, bool) {
	var nat tun.PacketNAT
	source := conn
	fakeIPConn, isFakeIP := conn.(*fakeIPNATPacketConn)
	if isFakeIP {
		source = fakeIPConn.NetPacketConn
	}
	spliceSource, isSpliceSource := unwrapSpliceSource(source)
	if !isSpliceSource {
		return conn, false
	}
	target, isTarget := unwrapSpliceTarget(remote, true)
	if !isTarget {
		return conn, false
	}
	if destinationAddress.IsValid() {
		nat.Destination = M.SocksaddrFrom(destinationAddress, metadata.Destination.Port)
	} else {
		nat.Destination = metadata.Destination
	}
	switch {
	case isFakeIP:
		nat.Origin = metadata.OriginDestination
		nat.FakeIP = true
	case metadata.RouteOriginalDestination.IsValid() && metadata.RouteOriginalDestination != metadata.Destination:
		nat.Origin = metadata.RouteOriginalDestination
	case destinationAddress.IsValid() && metadata.Destination.IsIP():
		nat.Origin = metadata.Destination
	}
	nat.Unidirectional = !isFakeIP && metadata.UDPDisableDomainUnmapping && !metadata.Destination.IsIP()
	cached := spliceSource.takeCached()
	if spliceSource.natConn.Splice(target.socket, tun.SplicePacketOptions{
		SpliceOptions: tun.SpliceOptions{
			ReadCounters:  append(spliceSource.readCounters, target.writeCounters...),
			WriteCounters: append(target.readCounters, spliceSource.writeCounters...),
			OnClose:       m.spliceClose(ctx, conn, remote.(io.Closer), onClose),
		},
		Timeout:       udpTimeout,
		NAT:           nat,
		Cached:        cached,
		Offload:       target.offload,
		FrontHeadroom: N.CalculateFrontHeadroom(remote),
		RearHeadroom:  N.CalculateRearHeadroom(remote),
	}) {
		return conn, true
	}
	if len(cached) == 0 {
		return conn, false
	}
	for _, packet := range slices.Backward(cached) {
		for _, counter := range spliceSource.readCounters {
			counter(int64(packet.Buffer.Len()))
		}
		source = bufio.NewCachedPacketConn(source, packet.Buffer, packet.Destination)
		N.PutPacketBuffer(packet)
	}
	if isFakeIP {
		fakeIPConn.NetPacketConn = bufio.NewNetPacketConn(source)
		return conn, false
	}
	return source, false
}

func packetTimeout(metadata *adapter.InboundContext) time.Duration {
	if metadata.UDPTimeout > 0 {
		return metadata.UDPTimeout
	}
	protocol := metadata.Protocol
	if protocol == "" {
		protocol = C.PortProtocols[metadata.Destination.Port]
	}
	if protocol != "" {
		return C.ProtocolTimeouts[protocol]
	}
	return 0
}

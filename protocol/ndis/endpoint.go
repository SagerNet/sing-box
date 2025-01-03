//go:build windows

package ndis

import (
	"sync"

	"github.com/sagernet/gvisor/pkg/buffer"
	"github.com/sagernet/gvisor/pkg/tcpip"
	"github.com/sagernet/gvisor/pkg/tcpip/header"
	"github.com/sagernet/gvisor/pkg/tcpip/stack"

	"github.com/wiresock/ndisapi-go"
	"github.com/wiresock/ndisapi-go/driver"
)

var _ stack.LinkEndpoint = (*ndisEndpoint)(nil)

type ndisEndpoint struct {
	filter     *driver.QueuedPacketFilter
	mtu        uint32
	address    tcpip.LinkAddress
	dispatcher stack.NetworkDispatcher
}

func (e *ndisEndpoint) MTU() uint32 {
	return e.mtu
}

func (e *ndisEndpoint) SetMTU(mtu uint32) {
}

func (e *ndisEndpoint) MaxHeaderLength() uint16 {
	return header.EthernetMinimumSize
}

func (e *ndisEndpoint) LinkAddress() tcpip.LinkAddress {
	return e.address
}

func (e *ndisEndpoint) SetLinkAddress(addr tcpip.LinkAddress) {
}

func (e *ndisEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return 0
}

func (e *ndisEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcher = dispatcher
}

func (e *ndisEndpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *ndisEndpoint) Wait() {
}

func (e *ndisEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareEther
}

func (e *ndisEndpoint) AddHeader(pkt *stack.PacketBuffer) {
	eth := header.Ethernet(pkt.LinkHeader().Push(header.EthernetMinimumSize))
	fields := header.EthernetFields{
		SrcAddr: pkt.EgressRoute.LocalLinkAddress,
		DstAddr: pkt.EgressRoute.RemoteLinkAddress,
		Type:    pkt.NetworkProtocolNumber,
	}
	eth.Encode(&fields)
}

func (e *ndisEndpoint) ParseHeader(pkt *stack.PacketBuffer) bool {
	_, ok := pkt.LinkHeader().Consume(header.EthernetMinimumSize)
	return ok
}

func (e *ndisEndpoint) Close() {
}

func (e *ndisEndpoint) SetOnCloseAction(f func()) {
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(ndisapi.IntermediateBuffer)
	},
}

func (e *ndisEndpoint) WritePackets(list stack.PacketBufferList) (int, tcpip.Error) {
	for _, packetBuffer := range list.AsSlice() {
		ndisBuf := bufferPool.Get().(*ndisapi.IntermediateBuffer)
		viewList, offset := packetBuffer.AsViewList()
		var view *buffer.View
		for view = viewList.Front(); view != nil && offset >= view.Size(); view = view.Next() {
			offset -= view.Size()
		}
		index := copy(ndisBuf.Buffer[:], view.AsSlice()[offset:])
		for view = view.Next(); view != nil; view = view.Next() {
			index += copy(ndisBuf.Buffer[index:], view.AsSlice())
		}
		ndisBuf.Length = uint32(index)
		err := e.filter.InsertPacketToMstcp(ndisBuf)
		bufferPool.Put(ndisBuf)
		if err != nil {
			return 0, &tcpip.ErrAborted{}
		}
	}
	return list.Len(), nil
}

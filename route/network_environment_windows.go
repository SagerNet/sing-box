package route

import (
	"net"
	"net/netip"
	"slices"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func systemGateways(interfaceIndex int) []netip.Addr {
	bufferSize := uint32(15000)
	var buffer []byte
	for {
		buffer = make([]byte, bufferSize)
		const flags = windows.GAA_FLAG_INCLUDE_GATEWAYS |
			windows.GAA_FLAG_SKIP_ANYCAST |
			windows.GAA_FLAG_SKIP_MULTICAST |
			windows.GAA_FLAG_SKIP_DNS_SERVER
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0])), &bufferSize)
		if err == nil {
			break
		}
		if err != windows.ERROR_BUFFER_OVERFLOW || bufferSize <= uint32(len(buffer)) {
			return nil
		}
	}
	var gateways []netip.Addr
	for adapter := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0])); adapter != nil; adapter = adapter.Next {
		if int(adapter.IfIndex) != interfaceIndex && int(adapter.Ipv6IfIndex) != interfaceIndex {
			continue
		}
		for gatewayAddress := adapter.FirstGatewayAddress; gatewayAddress != nil; gatewayAddress = gatewayAddress.Next {
			gateway, valid := netip.AddrFromSlice(gatewayAddress.Address.IP())
			if valid {
				gateways = append(gateways, gateway.Unmap().WithZone(""))
			}
		}
	}
	return gateways
}

var (
	modiphlpapi        = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetIpNetTable2 = modiphlpapi.NewProc("GetIpNetTable2")
	procFreeMibTable   = modiphlpapi.NewProc("FreeMibTable")
)

const (
	neighborStateUnreachable = 0
	neighborStateIncomplete  = 1
)

type mibIPNetRow2 struct {
	Address               windows.RawSockaddrInet6
	InterfaceIndex        uint32
	InterfaceLUID         uint64
	PhysicalAddress       [32]byte
	PhysicalAddressLength uint32
	State                 uint32
	Flags                 uint8
	_                     [3]byte
	ReachabilityTime      uint32
}

type mibIPNetTable2 struct {
	NumEntries uint32
	_          [4]byte
	Table      [1]mibIPNetRow2
}

func systemNeighborHardwareAddresses(interfaceIndex int, addresses []netip.Addr) map[netip.Addr]net.HardwareAddr {
	var table *mibIPNetTable2
	result, _, _ := procGetIpNetTable2.Call(uintptr(syscall.AF_UNSPEC), uintptr(unsafe.Pointer(&table)))
	if result != 0 || table == nil {
		return nil
	}
	defer procFreeMibTable.Call(uintptr(unsafe.Pointer(table)))
	rows := unsafe.Slice(&table.Table[0], table.NumEntries)
	hardwareAddresses := make(map[netip.Addr]net.HardwareAddr)
	for i := range rows {
		row := &rows[i]
		if int(row.InterfaceIndex) != interfaceIndex {
			continue
		}
		if row.State == neighborStateUnreachable || row.State == neighborStateIncomplete {
			continue
		}
		if row.PhysicalAddressLength == 0 || row.PhysicalAddressLength > uint32(len(row.PhysicalAddress)) {
			continue
		}
		var rowAddress netip.Addr
		switch row.Address.Family {
		case windows.AF_INET:
			rowAddress = netip.AddrFrom4((*windows.RawSockaddrInet4)(unsafe.Pointer(&row.Address)).Addr)
		case windows.AF_INET6:
			rowAddress = netip.AddrFrom16(row.Address.Addr)
		default:
			continue
		}
		if !slices.Contains(addresses, rowAddress) {
			continue
		}
		hardwareAddress := make(net.HardwareAddr, row.PhysicalAddressLength)
		copy(hardwareAddress, row.PhysicalAddress[:row.PhysicalAddressLength])
		hardwareAddresses[rowAddress] = hardwareAddress
	}
	return hardwareAddresses
}

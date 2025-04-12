package local

import (
	"context"
	"net"
	"net/netip"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/service"

	"golang.org/x/sys/windows"
)

func dnsReadConfig(ctx context.Context, _ string) *dnsConfig {
	conf := &dnsConfig{
		ndots:    1,
		timeout:  5 * time.Second,
		attempts: 2,
	}
	defer func() {
		if len(conf.servers) == 0 {
			conf.servers = defaultNS
		}
	}()
	addresses, err := adapterAddresses()
	if err != nil {
		return nil
	}
	var dnsAddresses []struct {
		ifName string
		netip.Addr
	}
	for _, address := range addresses {
		if address.OperStatus != windows.IfOperStatusUp {
			continue
		}
		if address.IfType == windows.IF_TYPE_TUNNEL {
			continue
		}
		if address.FirstGatewayAddress == nil {
			continue
		}
		for dnsServerAddress := address.FirstDnsServerAddress; dnsServerAddress != nil; dnsServerAddress = dnsServerAddress.Next {
			rawSockaddr, err := dnsServerAddress.Address.Sockaddr.Sockaddr()
			if err != nil {
				continue
			}
			var dnsServerAddr netip.Addr
			switch sockaddr := rawSockaddr.(type) {
			case *syscall.SockaddrInet4:
				dnsServerAddr = netip.AddrFrom4(sockaddr.Addr)
			case *syscall.SockaddrInet6:
				if sockaddr.Addr[0] == 0xfe && sockaddr.Addr[1] == 0xc0 {
					// fec0/10 IPv6 addresses are site local anycast DNS
					// addresses Microsoft sets by default if no other
					// IPv6 DNS address is set. Site local anycast is
					// deprecated since 2004, see
					// https://datatracker.ietf.org/doc/html/rfc3879
					continue
				}
				dnsServerAddr = netip.AddrFrom16(sockaddr.Addr)
			default:
				// Unexpected type.
				continue
			}
			dnsAddresses = append(dnsAddresses, struct {
				ifName string
				netip.Addr
			}{ifName: windows.UTF16PtrToString(address.FriendlyName), Addr: dnsServerAddr})
		}
	}
	var myInterface string
	if networkManager := service.FromContext[adapter.NetworkManager](ctx); networkManager != nil {
		myInterface = networkManager.InterfaceMonitor().MyInterface()
	}
	for _, address := range dnsAddresses {
		if address.ifName == myInterface {
			continue
		}
		conf.servers = append(conf.servers, net.JoinHostPort(address.String(), "53"))
	}
	return conf
}

func adapterAddresses() ([]*windows.IpAdapterAddresses, error) {
	var b []byte
	l := uint32(15000) // recommended initial size
	for {
		b = make([]byte, l)
		const flags = windows.GAA_FLAG_INCLUDE_PREFIX | windows.GAA_FLAG_INCLUDE_GATEWAYS
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return nil, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return nil, os.NewSyscallError("getadaptersaddresses", err)
		}
	}
	var aas []*windows.IpAdapterAddresses
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		aas = append(aas, aa)
	}
	return aas, nil
}

package systemconfig

import (
	"context"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"golang.org/x/sys/windows"
)

type Source struct {
	interfaceMonitor tun.DefaultInterfaceMonitor
	access           sync.Mutex
	updateCallback   *list.Element[tun.DefaultInterfaceUpdateCallback]
	stale            bool
	config           *Config
}

func NewSource(ctx context.Context) *Source {
	source := &Source{}
	interfaceMonitor := service.FromContext[adapter.NetworkManager](ctx).InterfaceMonitor()
	if interfaceMonitor != nil {
		source.interfaceMonitor = interfaceMonitor
		source.updateCallback = interfaceMonitor.RegisterCallback(source.interfaceUpdated)
	}
	return source
}

func (s *Source) Configuration() *Config {
	s.access.Lock()
	defer s.access.Unlock()
	if s.config != nil && !s.stale && s.updateCallback != nil {
		return s.config
	}
	s.stale = false
	config := s.readConfig()
	if s.config != nil && config.Equal(s.config) {
		return s.config
	}
	s.config = config
	return config
}

func (s *Source) interfaceUpdated(defaultInterface *control.Interface, flags int) {
	s.access.Lock()
	s.stale = true
	s.access.Unlock()
}

func (s *Source) Reset() {
	s.access.Lock()
	s.stale = true
	s.access.Unlock()
}

func (s *Source) Close() error {
	s.access.Lock()
	updateCallback := s.updateCallback
	s.updateCallback = nil
	s.access.Unlock()
	if updateCallback != nil {
		s.interfaceMonitor.UnregisterCallback(updateCallback)
	}
	return nil
}

func (s *Source) readConfig() *Config {
	config := &Config{
		Ndots:    1,
		Timeout:  5 * time.Second,
		Attempts: 2,
	}
	defer func() {
		if len(config.Servers) == 0 {
			config.Servers = defaultServers
		}
		if len(config.Search) == 0 {
			config.Search = defaultSearch()
		}
	}()
	addresses, err := adapterAddresses()
	if err != nil {
		return config
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
			rawSockaddr, sockaddrErr := dnsServerAddress.Address.Sockaddr.Sockaddr()
			if sockaddrErr != nil {
				continue
			}
			var dnsServerAddr netip.Addr
			switch sockaddr := rawSockaddr.(type) {
			case *syscall.SockaddrInet4:
				dnsServerAddr = netip.AddrFrom4(sockaddr.Addr)
			case *syscall.SockaddrInet6:
				if sockaddr.Addr[0] == 0xfe && sockaddr.Addr[1] == 0xc0 {
					// fec0::/10 site local anycast addresses are set by
					// Windows itself when no IPv6 DNS server is configured.
					continue
				}
				dnsServerAddr = netip.AddrFrom16(sockaddr.Addr)
				if sockaddr.ZoneId != 0 {
					dnsServerAddr = dnsServerAddr.WithZone(strconv.FormatInt(int64(sockaddr.ZoneId), 10))
				}
			default:
				continue
			}
			dnsAddresses = append(dnsAddresses, struct {
				ifName string
				netip.Addr
			}{ifName: windows.UTF16PtrToString(address.FriendlyName), Addr: dnsServerAddr})
		}
	}
	var myInterfaces []string
	if s.interfaceMonitor != nil {
		myInterfaces = s.interfaceMonitor.MyInterfaces()
	}
	var servers []M.Socksaddr
	for _, address := range dnsAddresses {
		if slices.Contains(myInterfaces, address.ifName) {
			continue
		}
		servers = append(servers, M.SocksaddrFrom(address.Addr, 53))
	}
	config.Servers = common.Uniq(servers)
	return config
}

func adapterAddresses() ([]*windows.IpAdapterAddresses, error) {
	var b []byte
	l := uint32(15000)
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

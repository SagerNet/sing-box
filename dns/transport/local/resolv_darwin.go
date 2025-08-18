package local

import (
	"context"
	"net/netip"
	"syscall"
	"time"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/miekg/dns"
)

func dnsReadConfig(_ context.Context, _ string) *dnsConfig {
	resStateSize := unsafe.Sizeof(_C_struct___res_state{})
	var state *_C_struct___res_state
	if resStateSize > 0 {
		mem := _C_malloc(resStateSize)
		defer _C_free(mem)
		memSlice := unsafe.Slice((*byte)(mem), resStateSize)
		clear(memSlice)
		state = (*_C_struct___res_state)(unsafe.Pointer(&memSlice[0]))
	}
	if err := ResNinit(state); err != nil {
		return &dnsConfig{
			servers:  defaultNS,
			search:   dnsDefaultSearch(),
			ndots:    1,
			timeout:  5 * time.Second,
			attempts: 2,
			err:      E.Cause(err, "libresolv initialization failed"),
		}
	}
	defer ResNclose(state)
	conf := &dnsConfig{
		ndots:    1,
		timeout:  5 * time.Second,
		attempts: int(state.Retry),
	}
	for i := 0; i < int(state.Nscount); i++ {
		addr := parseRawSockaddr(&state.Nsaddrlist[i])
		if addr.IsValid() {
			conf.servers = append(conf.servers, addr.String())
		}
	}
	for i := 0; ; i++ {
		search := state.Dnsrch[i]
		if search == nil {
			break
		}
		name := dns.Fqdn(GoString(search))
		if name == "" {
			continue
		}
		conf.search = append(conf.search, name)
	}
	return conf
}

func parseRawSockaddr(rawSockaddr *syscall.RawSockaddr) netip.Addr {
	switch rawSockaddr.Family {
	case syscall.AF_INET:
		sa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(rawSockaddr))
		return netip.AddrFrom4(sa.Addr)
	case syscall.AF_INET6:
		sa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(rawSockaddr))
		return netip.AddrFrom16(sa.Addr)
	default:
		return netip.Addr{}
	}
}

//go:build darwin && cgo

package local

/*
#include <stdlib.h>
#include <stdio.h>
#include <resolv.h>
#include <arpa/inet.h>
*/
import "C"

import (
	"context"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/miekg/dns"
)

func dnsReadConfig(_ context.Context, _ string) *dnsConfig {
	var state C.struct___res_state
	if C.res_ninit(&state) != 0 {
		return &dnsConfig{
			servers:  defaultNS,
			search:   dnsDefaultSearch(),
			ndots:    1,
			timeout:  5 * time.Second,
			attempts: 2,
			err:      E.New("libresolv initialization failed"),
		}
	}
	conf := &dnsConfig{
		ndots:    1,
		timeout:  5 * time.Second,
		attempts: int(state.retry),
	}
	for i := 0; i < int(state.nscount); i++ {
		ns := state.nsaddr_list[i]
		addr := C.inet_ntoa(ns.sin_addr)
		if addr == nil {
			continue
		}
		conf.servers = append(conf.servers, C.GoString(addr))
	}
	for i := 0; ; i++ {
		search := state.dnsrch[i]
		if search == nil {
			break
		}
		conf.search = append(conf.search, dns.Fqdn(C.GoString(search)))
	}
	return conf
}

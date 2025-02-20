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
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/miekg/dns"
)

func dnsReadConfig(_ string) *dnsConfig {
	if C.res_init() != 0 {
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
		attempts: int(C._res.retry),
	}
	for i := 0; i < int(C._res.nscount); i++ {
		ns := C._res.nsaddr_list[i]
		addr := C.inet_ntoa(ns.sin_addr)
		if addr == nil {
			continue
		}
		conf.servers = append(conf.servers, C.GoString(addr))
	}
	for i := 0; ; i++ {
		search := C._res.dnsrch[i]
		if search == nil {
			break
		}
		conf.search = append(conf.search, dns.Fqdn(C.GoString(search)))
	}
	return conf
}

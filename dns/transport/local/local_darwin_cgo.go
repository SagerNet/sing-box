//go:build darwin

package local

/*
#include <stdlib.h>
#include <netdb.h>
#include <dns.h>

static int cgo_dns_search(const char *name, int class, int type,
	unsigned char *answer, int anslen, int *out_h_errno) {
	dns_handle_t handle = (dns_handle_t)dns_open(NULL);
	if (handle == NULL) {
		*out_h_errno = NO_RECOVERY;
		return -1;
	}
	struct sockaddr_storage from;
	uint32_t fromlen = sizeof(from);
	h_errno = 0;
	int n = dns_search(handle, name, class, type, (char *)answer, anslen,
		(struct sockaddr *)&from, &fromlen);
	*out_h_errno = h_errno;
	dns_free(handle);
	return n;
}
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"

	boxC "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

const (
	darwinResolverHostNotFound = 1
	darwinResolverTryAgain     = 2
	darwinResolverNoRecovery   = 3
	darwinResolverNoData       = 4
)

func darwinLookupSystemDNS(name string, class, qtype int) (*mDNS.Msg, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	answer := make([]byte, 4096)
	var hErrno C.int
	n := C.cgo_dns_search(cName, C.int(class), C.int(qtype),
		(*C.uchar)(unsafe.Pointer(&answer[0])), C.int(len(answer)),
		&hErrno)
	if n <= 0 {
		return nil, darwinResolverHErrno(name, int(hErrno))
	}
	var response mDNS.Msg
	err := response.Unpack(answer[:int(n)])
	if err != nil {
		return nil, E.Cause(err, "unpack dns_search response")
	}
	return &response, nil
}

func darwinResolverHErrno(name string, hErrno int) error {
	switch hErrno {
	case darwinResolverHostNotFound:
		return dns.RcodeNameError
	case darwinResolverNoData:
		return dns.RcodeSuccess
	case darwinResolverTryAgain, darwinResolverNoRecovery:
		return dns.RcodeServerFailure
	default:
		return E.New("dns_search: unknown h_errno ", hErrno, " for ", name)
	}
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	if t.hosts != nil && (question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA) {
		addresses := t.hosts.Lookup(dns.FqdnToDomain(question.Name))
		if len(addresses) > 0 {
			return dns.FixedResponse(message.Id, question, addresses, boxC.DefaultDNSTTL), nil
		}
	}
	if t.fallback && t.dhcpTransport != nil {
		dhcpServers := t.dhcpTransport.Fetch()
		if len(dhcpServers) > 0 {
			return t.dhcpTransport.Exchange0(ctx, message, dhcpServers)
		}
	}
	type resolvResult struct {
		response *mDNS.Msg
		err      error
	}
	resultCh := make(chan resolvResult, 1)
	go func() {
		response, err := darwinLookupSystemDNS(question.Name, int(question.Qclass), int(question.Qtype))
		resultCh <- resolvResult{response, err}
	}()
	var result resolvResult
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result = <-resultCh:
	}
	if result.err != nil {
		var rcodeError dns.RcodeError
		if errors.As(result.err, &rcodeError) {
			return dns.FixedResponseStatus(message, int(rcodeError)), nil
		}
		return nil, result.err
	}
	result.response.Id = message.Id
	// Workaround for a bug in Apple libresolv: res_query_mDNSResponder
	// (libresolv/res_query.c), used when the resolver has
	// DNS_FLAG_FORWARD_TO_MDNSRESPONDER set (typical inside a Network
	// Extension), writes:
	//
	//     ans->qr = 1;
	//     ans->qr = htons(ans->qr);
	//
	// HEADER.qr is a 1-bit bitfield (<arpa/nameser_compat.h>), so
	// htons(1) == 0x0100 gets truncated back to 0, clearing the QR bit.
	// Force it on so downstream clients see a valid response.
	result.response.Response = true
	return result.response, nil
}

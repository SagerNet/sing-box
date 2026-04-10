//go:build darwin

package local

/*
#include <stdlib.h>
#include <dns.h>
#include <resolv.h>

static void *cgo_dns_open_super() {
	return (void *)dns_open(NULL);
}

static void cgo_dns_close(void *opaque) {
	if (opaque != NULL) dns_free((dns_handle_t)opaque);
}

static int cgo_dns_search(void *opaque, const char *name, int class, int type,
	unsigned char *answer, int anslen) {
	dns_handle_t handle = (dns_handle_t)opaque;
	struct sockaddr_storage from;
	uint32_t fromlen = sizeof(from);
	return dns_search(handle, name, class, type, (char *)answer, anslen, (struct sockaddr *)&from, &fromlen);
}

static void *cgo_res_init() {
	res_state state = calloc(1, sizeof(struct __res_state));
	if (state == NULL) return NULL;
	if (res_ninit(state) != 0) {
		free(state);
		return NULL;
	}
	return state;
}

static void cgo_res_destroy(void *opaque) {
	res_state state = (res_state)opaque;
	res_ndestroy(state);
	free(state);
}

static int cgo_res_nsearch(void *opaque, const char *dname, int class, int type,
	unsigned char *answer, int anslen,
	int timeout_seconds,
	int *out_h_errno) {
	res_state state = (res_state)opaque;
	state->retrans = timeout_seconds;
	state->retry = 1;
	int n = res_nsearch(state, dname, class, type, answer, anslen);
	if (n < 0) {
		*out_h_errno = state->res_h_errno;
	}
	return n;
}
*/
import "C"

import (
	"context"
	"errors"
	"time"
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

	darwinResolverMaxPacketSize = 65535
)

var errDarwinNeedLargerBuffer = errors.New("darwin resolver response truncated")

func darwinLookupSystemDNS(name string, class, qtype, timeoutSeconds int) (*mDNS.Msg, error) {
	response, err := darwinSearchWithSystemRouting(name, class, qtype)
	if err == nil {
		return response, nil
	}
	fallbackResponse, fallbackErr := darwinSearchWithResolv(name, class, qtype, timeoutSeconds)
	if fallbackErr == nil || fallbackResponse != nil {
		return fallbackResponse, fallbackErr
	}
	return nil, E.Errors(
		E.Cause(err, "dns_search"),
		E.Cause(fallbackErr, "res_nsearch"),
	)
}

func darwinSearchWithSystemRouting(name string, class, qtype int) (*mDNS.Msg, error) {
	handle := C.cgo_dns_open_super()
	if handle == nil {
		return nil, E.New("dns_open failed")
	}
	defer C.cgo_dns_close(handle)

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	bufSize := 1232
	for {
		answer := make([]byte, bufSize)
		n := C.cgo_dns_search(handle, cName, C.int(class), C.int(qtype),
			(*C.uchar)(unsafe.Pointer(&answer[0])), C.int(len(answer)))
		if n <= 0 {
			return nil, E.New("dns_search failed for ", name)
		}
		if int(n) > bufSize {
			bufSize = int(n)
			continue
		}
		return unpackDarwinResolverMessage(answer[:int(n)], "dns_search")
	}
}

func darwinSearchWithResolv(name string, class, qtype int, timeoutSeconds int) (*mDNS.Msg, error) {
	state := C.cgo_res_init()
	if state == nil {
		return nil, E.New("res_ninit failed")
	}
	defer C.cgo_res_destroy(state)

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	bufSize := 1232
	for {
		answer := make([]byte, bufSize)
		var hErrno C.int
		n := C.cgo_res_nsearch(state, cName, C.int(class), C.int(qtype),
			(*C.uchar)(unsafe.Pointer(&answer[0])), C.int(len(answer)),
			C.int(timeoutSeconds),
			&hErrno)
		if n >= 0 {
			if int(n) > bufSize {
				bufSize = int(n)
				continue
			}
			return unpackDarwinResolverMessage(answer[:int(n)], "res_nsearch")
		}
		response, err := handleDarwinResolvFailure(name, answer, int(hErrno))
		if err == nil {
			return response, nil
		}
		if errors.Is(err, errDarwinNeedLargerBuffer) && bufSize < darwinResolverMaxPacketSize {
			bufSize *= 2
			if bufSize > darwinResolverMaxPacketSize {
				bufSize = darwinResolverMaxPacketSize
			}
			continue
		}
		return nil, err
	}
}

func unpackDarwinResolverMessage(packet []byte, source string) (*mDNS.Msg, error) {
	var response mDNS.Msg
	err := response.Unpack(packet)
	if err != nil {
		return nil, E.Cause(err, "unpack ", source, " response")
	}
	return &response, nil
}

func handleDarwinResolvFailure(name string, answer []byte, hErrno int) (*mDNS.Msg, error) {
	response, err := unpackDarwinResolverMessage(answer, "res_nsearch failure")
	if err == nil && response.Response {
		if response.Truncated && len(answer) < darwinResolverMaxPacketSize {
			return nil, errDarwinNeedLargerBuffer
		}
		return response, nil
	}
	return nil, darwinResolverHErrno(name, hErrno)
}

func darwinResolverHErrno(name string, hErrno int) error {
	switch hErrno {
	case darwinResolverHostNotFound:
		return dns.RcodeNameError
	case darwinResolverTryAgain:
		return dns.RcodeServerFailure
	case darwinResolverNoRecovery:
		return dns.RcodeServerFailure
	case darwinResolverNoData:
		return dns.RcodeSuccess
	default:
		return E.New("res_nsearch: unknown error ", hErrno, " for ", name)
	}
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
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
	name := question.Name
	timeoutSeconds := int(boxC.DNSTimeout / time.Second)
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		seconds := int(remaining.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		timeoutSeconds = seconds
	}
	type resolvResult struct {
		response *mDNS.Msg
		err      error
	}
	resultCh := make(chan resolvResult, 1)
	go func() {
		response, err := darwinLookupSystemDNS(name, int(question.Qclass), int(question.Qtype), timeoutSeconds)
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
	return result.response, nil
}

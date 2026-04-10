//go:build darwin

package local

/*
#include <stdlib.h>
#include <resolv.h>
#include <netdb.h>

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

func resolvSearch(name string, class, qtype int, timeoutSeconds int) (*mDNS.Msg, error) {
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
			var response mDNS.Msg
			err := response.Unpack(answer[:int(n)])
			if err != nil {
				return nil, E.Cause(err, "unpack res_nsearch response")
			}
			return &response, nil
		}
		var response mDNS.Msg
		_ = response.Unpack(answer[:bufSize])
		if response.Response {
			if response.Truncated && bufSize < 65535 {
				bufSize *= 2
				if bufSize > 65535 {
					bufSize = 65535
				}
				continue
			}
			return &response, nil
		}
		switch hErrno {
		case C.HOST_NOT_FOUND:
			return nil, dns.RcodeNameError
		case C.TRY_AGAIN:
			return nil, dns.RcodeNameError
		case C.NO_RECOVERY:
			return nil, dns.RcodeServerFailure
		case C.NO_DATA:
			return nil, dns.RcodeSuccess
		default:
			return nil, E.New("res_nsearch: unknown error ", int(hErrno), " for ", name)
		}
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
		response, err := resolvSearch(name, int(question.Qclass), int(question.Qtype), timeoutSeconds)
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

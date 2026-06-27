//go:build darwin

package local

import (
	"cmp"
	"context"
	"net"
	"os"
	"testing"
	"time"

	mDNS "github.com/miekg/dns"
)

// "localhost" is answered by the mDNSResponder daemon itself, so these tests need
// no external network.

func requireMDNSResponder(t *testing.T) {
	t.Helper()
	socketPath := cmp.Or(os.Getenv(mdnsResponderSocketEnv), mdnsResponderSocketPath)
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Skipf("mDNSResponder not reachable at %s: %v", socketPath, err)
	}
	conn.Close()
}

func TestSystemExchangeLoopback(t *testing.T) {
	requireMDNSResponder(t)
	transport := &Transport{}
	for _, testCase := range []struct {
		qtype    uint16
		expected net.IP
	}{
		{mDNS.TypeA, net.IPv4(127, 0, 0, 1)},
		{mDNS.TypeAAAA, net.IPv6loopback},
	} {
		message := new(mDNS.Msg)
		message.SetQuestion("localhost.", testCase.qtype)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		response, err := transport.systemExchange(ctx, message)
		cancel()
		if err != nil {
			t.Fatalf("%s localhost: %v", mDNS.TypeToString[testCase.qtype], err)
		}
		if response.Id != message.Id {
			t.Fatalf("%s response id %d != request id %d", mDNS.TypeToString[testCase.qtype], response.Id, message.Id)
		}
		if !response.Response {
			t.Fatalf("%s response flag not set", mDNS.TypeToString[testCase.qtype])
		}
		var found bool
		for _, answer := range response.Answer {
			switch record := answer.(type) {
			case *mDNS.A:
				found = found || record.A.Equal(testCase.expected)
			case *mDNS.AAAA:
				found = found || record.AAAA.Equal(testCase.expected)
			}
		}
		if !found {
			t.Fatalf("%s localhost: expected %s in answer, got %v", mDNS.TypeToString[testCase.qtype], testCase.expected, response.Answer)
		}
	}
}

func TestSystemExchangeNoData(t *testing.T) {
	requireMDNSResponder(t)
	message := new(mDNS.Msg)
	// localhost has no MX record, so the daemon reports NoSuchRecord, which must
	// surface as an empty NOERROR response rather than an error.
	message.SetQuestion("localhost.", mDNS.TypeMX)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := (&Transport{}).systemExchange(ctx, message)
	if err != nil {
		t.Fatalf("MX localhost: %v", err)
	}
	if response.Rcode != mDNS.RcodeSuccess {
		t.Fatalf("MX localhost: rcode %s, want NOERROR", mDNS.RcodeToString[response.Rcode])
	}
	if len(response.Answer) != 0 {
		t.Fatalf("MX localhost: expected no answers, got %v", response.Answer)
	}
}

func TestSystemExchangeCancel(t *testing.T) {
	requireMDNSResponder(t)
	message := new(mDNS.Msg)
	message.SetQuestion("localhost.", mDNS.TypeA)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := (&Transport{}).systemExchange(ctx, message)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if elapsed > time.Second {
		t.Fatalf("cancellation too slow: %s", elapsed)
	}
}

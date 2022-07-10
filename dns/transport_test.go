package dns_test

import (
	"context"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestTCPDNS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	transport := dns.NewTCPTransport(ctx, N.SystemDialer, log.NewNopLogger(), M.ParseSocksaddr("1.0.0.1:53"))
	response, err := transport.Exchange(ctx, makeQuery())
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	for _, answer := range response.Answers {
		t.Log(answer)
	}
}

func TestTLSDNS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	transport := dns.NewTLSTransport(ctx, N.SystemDialer, log.NewNopLogger(), M.ParseSocksaddr("1.0.0.1:853"))
	response, err := transport.Exchange(ctx, makeQuery())
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	for _, answer := range response.Answers {
		t.Log(answer)
	}
}

func TestHTTPSDNS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	transport := dns.NewHTTPSTransport(N.SystemDialer, "https://1.0.0.1:443/dns-query")
	response, err := transport.Exchange(ctx, makeQuery())
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	for _, answer := range response.Answers {
		t.Log(answer)
	}
}

func TestUDPDNS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	transport := dns.NewUDPTransport(ctx, N.SystemDialer, log.NewNopLogger(), M.ParseSocksaddr("1.0.0.1:53"))
	response, err := transport.Exchange(ctx, makeQuery())
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	for _, answer := range response.Answers {
		t.Log(answer)
	}
}

func TestLocalDNS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	transport := dns.NewLocalTransport()
	response, err := transport.Lookup(ctx, "google.com", C.DomainStrategyAsIS)
	cancel()
	require.NoError(t, err)
	require.NotEmpty(t, response, "no answers")
	for _, answer := range response {
		t.Log(answer)
	}
}

func makeQuery() *dnsmessage.Message {
	message := &dnsmessage.Message{}
	message.Header.ID = 1
	message.Header.RecursionDesired = true
	message.Questions = append(message.Questions, dnsmessage.Question{
		Name:  dnsmessage.MustNewName("google.com."),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	})
	return message
}

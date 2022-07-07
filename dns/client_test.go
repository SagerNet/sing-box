package dns_test

import (
	"context"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	client := dns.NewClient(option.DNSClientOptions{})
	dnsTransport := dns.NewTCPTransport(context.Background(), N.SystemDialer, log.NewNopLogger(), M.ParseSocksaddr("1.0.0.1:53"))
	response, err := client.Exchange(ctx, dnsTransport, makeQuery())
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	response, err = client.Exchange(ctx, dnsTransport, makeQuery())
	require.NoError(t, err)
	require.NotEmpty(t, response.Answers, "no answers")
	addresses, err := client.Lookup(ctx, dnsTransport, "www.google.com", C.DomainStrategyAsIS)
	require.NoError(t, err)
	require.NotEmpty(t, addresses, "no answers")
	cancel()
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

package dhcp

import (
	"context"
	"math/rand"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

const (
	// net.maxDNSPacketSize
	maxDNSPacketSize = 1232
)

func (t *Transport) exchangeSingleRequest(ctx context.Context, servers []M.Socksaddr, message *mDNS.Msg, domain string) (*mDNS.Msg, error) {
	var lastErr error
	for _, fqdn := range t.nameList(domain) {
		response, err := t.tryOneName(ctx, servers, fqdn, message)
		if err != nil {
			lastErr = err
			continue
		}
		return response, nil
	}
	return nil, lastErr
}

func (t *Transport) exchangeParallel(ctx context.Context, servers []M.Socksaddr, message *mDNS.Msg, domain string) (*mDNS.Msg, error) {
	returned := make(chan struct{})
	defer close(returned)
	type queryResult struct {
		response *mDNS.Msg
		err      error
	}
	results := make(chan queryResult)
	startRacer := func(ctx context.Context, fqdn string) {
		response, err := t.tryOneName(ctx, servers, fqdn, message)
		if err == nil {
			if response.Rcode != mDNS.RcodeSuccess {
				err = dns.RcodeError(response.Rcode)
			} else if len(dns.MessageToAddresses(response)) == 0 {
				err = E.New(fqdn, ": empty result")
			}
		}
		select {
		case results <- queryResult{response, err}:
		case <-returned:
		}
	}
	queryCtx, queryCancel := context.WithCancel(ctx)
	defer queryCancel()
	var nameCount int
	for _, fqdn := range t.nameList(domain) {
		nameCount++
		go startRacer(queryCtx, fqdn)
	}
	var errors []error
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err == nil {
				return result.response, nil
			}
			errors = append(errors, result.err)
			if len(errors) == nameCount {
				return nil, E.Errors(errors...)
			}
		}
	}
}

func (t *Transport) tryOneName(ctx context.Context, servers []M.Socksaddr, fqdn string, message *mDNS.Msg) (*mDNS.Msg, error) {
	sLen := len(servers)
	var lastErr error
	for i := 0; i < t.attempts; i++ {
		for j := 0; j < sLen; j++ {
			server := servers[j]
			question := message.Question[0]
			question.Name = fqdn
			response, err := t.exchangeOne(ctx, server, question, C.DNSTimeout, false, true)
			if err != nil {
				lastErr = err
				continue
			}
			return response, nil
		}
	}
	return nil, E.Cause(lastErr, fqdn)
}

func (t *Transport) exchangeOne(ctx context.Context, server M.Socksaddr, question mDNS.Question, timeout time.Duration, useTCP, ad bool) (*mDNS.Msg, error) {
	if server.Port == 0 {
		server.Port = 53
	}
	var networks []string
	if useTCP {
		networks = []string{N.NetworkTCP}
	} else {
		networks = []string{N.NetworkUDP, N.NetworkTCP}
	}
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:                uint16(rand.Uint32()),
			RecursionDesired:  true,
			AuthenticatedData: ad,
		},
		Question: []mDNS.Question{question},
		Compress: true,
	}
	request.SetEdns0(maxDNSPacketSize, false)
	buffer := buf.Get(buf.UDPBufferSize)
	defer buf.Put(buffer)
	for _, network := range networks {
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(timeout))
		defer cancel()
		conn, err := t.dialer.DialContext(ctx, network, server)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		if deadline, loaded := ctx.Deadline(); loaded && !deadline.IsZero() {
			conn.SetDeadline(deadline)
		}
		rawMessage, err := request.PackBuffer(buffer)
		if err != nil {
			return nil, E.Cause(err, "pack request")
		}
		_, err = conn.Write(rawMessage)
		if err != nil {
			return nil, E.Cause(err, "write request")
		}
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, E.Cause(err, "read response")
		}
		var response mDNS.Msg
		err = response.Unpack(buffer[:n])
		if err != nil {
			return nil, E.Cause(err, "unpack response")
		}
		if response.Truncated && network == N.NetworkUDP {
			continue
		}
		return &response, nil
	}
	panic("unexpected")
}

func (t *Transport) nameList(name string) []string {
	l := len(name)
	rooted := l > 0 && name[l-1] == '.'
	if l > 254 || l == 254 && !rooted {
		return nil
	}

	if rooted {
		if avoidDNS(name) {
			return nil
		}
		return []string{name}
	}

	hasNdots := strings.Count(name, ".") >= t.ndots
	name += "."
	// l++

	names := make([]string, 0, 1+len(t.search))
	if hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	for _, suffix := range t.search {
		fqdn := name + suffix
		if !avoidDNS(fqdn) && len(fqdn) <= 254 {
			names = append(names, fqdn)
		}
	}
	if !hasNdots && !avoidDNS(name) {
		names = append(names, name)
	}
	return names
}

func avoidDNS(name string) bool {
	if name == "" {
		return true
	}
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return strings.HasSuffix(name, ".onion")
}

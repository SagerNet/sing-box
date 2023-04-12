package subscribe

import (
	"context"
	"fmt"
	mDNS "github.com/miekg/dns"
	dns "github.com/sagernet/sing-dns"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	queryTimeout  = 5 * time.Second
	DefaultUDPDNS = "223.5.5.5"
)

type DNS struct {
	ctx       context.Context
	transport dns.Transport
	dialer    N.Dialer
}

func NewDNS(ctx context.Context, addr string, dialer N.Dialer) (*DNS, error) {

	switch {
	case strings.Index(addr, "tcp://") == 0:
		// tcp dns
		addr = strings.TrimPrefix(addr, "tcp://")

		// check is ip
		ip, err := netip.ParseAddr(addr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewTCPTransport("dns-tcp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), "53")))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		// check is ip:port
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return nil, fmt.Errorf("invalid dns address: %s", "tcp://"+addr)
			}
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewTCPTransport("dns-tcp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), port)))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		return nil, fmt.Errorf("invalid dns address: %s", "tcp://"+addr)
	case strings.Index(addr, "udp://") == 0:
		// udp dns
		addr = strings.TrimPrefix(addr, "udp://")

		// check is ip
		ip, err := netip.ParseAddr(addr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewUDPTransport("dns-udp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), "53")))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		// check is ip:port
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return nil, fmt.Errorf("invalid dns address: %s", "udp://"+addr)
			}

			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewUDPTransport("dns-udp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), port)))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		return nil, fmt.Errorf("invalid dns address: %s", "udp://"+addr)
	case strings.Index(addr, "tls://") == 0:
		// dot dns
		addr = strings.TrimPrefix(addr, "tls://")

		// check is ip
		ip, err := netip.ParseAddr(addr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewTLSTransport("dns-dot", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), "853")))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		// check is ip:port
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return nil, fmt.Errorf("invalid dns address: %s", "tls://"+addr)
			}

			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewTLSTransport("dns-dot", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), port)))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		return nil, fmt.Errorf("invalid dns address: %s", "tls://"+addr)
	case strings.Index(addr, "https://") == 0:
		// doh dns
		u, err := url.Parse(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid dns address: %s, err: %s", addr, err)
		}

		if u.Fragment != "" || u.RawFragment != "" || u.RawQuery != "" {
			return nil, fmt.Errorf("invalid dns address: %s", addr)
		}

		hostAddr := u.Host

		// check is ip
		ip, err := netip.ParseAddr(hostAddr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			uNew := *u
			uNew.Host = net.JoinHostPort(ip.String(), "443")
			d.transport = dns.NewHTTPSTransport("dns-https", d.dialer, uNew.String())

			return d, nil
		}

		// check is ip:port
		host, port, err := net.SplitHostPort(hostAddr)
		if err == nil {
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return nil, fmt.Errorf("invalid dns address: %s", "https://"+addr)
			}

			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			uNew := *u
			uNew.Host = net.JoinHostPort(ip.String(), port)
			d.transport = dns.NewHTTPSTransport("dns-https", d.dialer, uNew.String())

			return d, nil
		}

		return nil, fmt.Errorf("invalid dns address: %s, domain is not supported", addr)
	case addr == "":
		ip, err := netip.ParseAddr(DefaultUDPDNS)
		if err != nil {
			return nil, fmt.Errorf("invalid dns address: %s", DefaultUDPDNS)
		}

		d := &DNS{}
		d.ctx = ctx
		d.dialer = dialer
		d.transport, err = dns.NewUDPTransport("dns-udp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), "53")))
		if err != nil {
			return nil, err
		}

		return d, nil
	default:
		// check is udp dns

		// check is ip
		ip, err := netip.ParseAddr(addr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewUDPTransport("dns-udp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(ip.String(), "53")))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		// check is ip:port
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			d := &DNS{}
			d.ctx = ctx
			d.dialer = dialer
			d.transport, err = dns.NewUDPTransport("dns-udp", ctx, d.dialer, M.ParseSocksaddr(net.JoinHostPort(host, port)))
			if err != nil {
				return nil, err
			}

			return d, nil
		}

		return nil, fmt.Errorf("invalid dns address: %s", addr)
	}
}

func (d *DNS) Query(msg *mDNS.Msg) (*mDNS.Msg, error) {
	ctx, cancel := context.WithTimeout(d.ctx, queryTimeout)
	defer cancel()

	return d.transport.Exchange(ctx, msg)
}

func (d *DNS) QueryTypeA(domain string) ([]string, error) {
	msg := new(mDNS.Msg)
	msg.SetQuestion(mDNS.Fqdn(domain), mDNS.TypeA)
	m, err := d.Query(msg)
	if err != nil {
		return nil, err
	}
	ips := make([]string, 0)
	for _, a := range m.Answer {
		if a.Header().Rrtype == mDNS.TypeA {
			ips = append(ips, a.(*mDNS.A).A.String())
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no A record")
	}
	return ips, nil
}

func (d *DNS) QueryTypeAAAA(domain string) ([]string, error) {
	msg := new(mDNS.Msg)
	msg.SetQuestion(mDNS.Fqdn(domain), mDNS.TypeAAAA)
	m, err := d.Query(msg)
	if err != nil {
		return nil, err
	}
	ips := make([]string, 0)
	for _, a := range m.Answer {
		if a.Header().Rrtype == mDNS.TypeAAAA {
			ips = append(ips, a.(*mDNS.AAAA).AAAA.String())
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no A record")
	}
	return ips, nil
}

func (d *DNS) QueryIP(domain string) ([]string, error) {
	wg := sync.WaitGroup{}
	ch := make(chan []string, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg, err := d.QueryTypeA(domain)
		if err != nil {
			return
		}
		ch <- msg
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg, err := d.QueryTypeAAAA(domain)
		if err != nil {
			return
		}
		ch <- msg
	}()
	wg.Wait()
	ips := make([]string, 0)
	for {
		select {
		case m := <-ch:
			ips = append(ips, m...)
		default:
			close(ch)
			if len(ips) == 0 {
				return nil, fmt.Errorf("no ip found")
			}
			return ips, nil
		}
	}
}

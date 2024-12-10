/*
Copyright and credits to @bepass-org [github.com/sagernet/sing-box]
*/

package ipscanner

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/sing-box/ipscanner/internal/engine"
	"github.com/sagernet/sing-box/ipscanner/internal/statute"
)

type IPInfo = statute.IPInfo

type IPScanner struct {
	log     *slog.Logger
	engine  *engine.Engine
	options statute.ScannerOptions
}

func NewScanner(options ...Option) *IPScanner {
	p := &IPScanner{
		options: statute.ScannerOptions{
			UseIPv4:            true,
			UseIPv6:            true,
			CidrList:           statute.DefaultCFRanges(),
			SelectedOps:        0,
			Logger:             slog.Default(),
			InsecureSkipVerify: true,
			RawDialerFunc:      statute.DefaultDialerFunc,
			TLSDialerFunc:      statute.DefaultTLSDialerFunc,
			HttpClientFunc:     statute.DefaultHTTPClientFunc,
			UseHTTP2:           false,
			DisableCompression: false,
			HTTPPath:           "/",
			Referrer:           "",
			UserAgent:          "Chrome/80.0.3987.149",
			Hostname:           "www.cloudflare.com",
			WarpPresharedKey:   "",
			WarpPeerPublicKey:  "",
			WarpPrivateKey:     "",
			Port:               443,
			IPQueueSize:        8,
			MaxDesirableRTT:    400 * time.Millisecond,
			IPQueueTTL:         30 * time.Second,
			ConnectionTimeout:  1 * time.Second,
			HandshakeTimeout:   1 * time.Second,
			TlsVersion:         tls.VersionTLS13,
		},
		log: slog.Default(),
	}

	for _, option := range options {
		option(p)
	}

	return p
}

type Option func(*IPScanner)

func WithUseIPv4(useIPv4 bool) Option {
	return func(i *IPScanner) {
		i.options.UseIPv4 = useIPv4
	}
}

func WithUseIPv6(useIPv6 bool) Option {
	return func(i *IPScanner) {
		i.options.UseIPv6 = useIPv6
	}
}

func WithDialer(d statute.TDialerFunc) Option {
	return func(i *IPScanner) {
		i.options.RawDialerFunc = d
	}
}

func WithTLSDialer(t statute.TDialerFunc) Option {
	return func(i *IPScanner) {
		i.options.TLSDialerFunc = t
	}
}

func WithHttpClientFunc(h statute.THTTPClientFunc) Option {
	return func(i *IPScanner) {
		i.options.HttpClientFunc = h
	}
}

func WithUseHTTP2(useHTTP2 bool) Option {
	return func(i *IPScanner) {
		i.options.UseHTTP2 = useHTTP2
	}
}

func WithDisableCompression(disableCompression bool) Option {
	return func(i *IPScanner) {
		i.options.DisableCompression = disableCompression
	}
}

func WithHttpPath(path string) Option {
	return func(i *IPScanner) {
		i.options.HTTPPath = path
	}
}

func WithReferrer(referrer string) Option {
	return func(i *IPScanner) {
		i.options.Referrer = referrer
	}
}

func WithUserAgent(userAgent string) Option {
	return func(i *IPScanner) {
		i.options.UserAgent = userAgent
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(i *IPScanner) {
		i.log = logger
		i.options.Logger = logger
	}
}

func WithInsecureSkipVerify(insecureSkipVerify bool) Option {
	return func(i *IPScanner) {
		i.options.InsecureSkipVerify = insecureSkipVerify
	}
}

func WithHostname(hostname string) Option {
	return func(i *IPScanner) {
		i.options.Hostname = hostname
	}
}

func WithPort(port uint16) Option {
	return func(i *IPScanner) {
		i.options.Port = port
	}
}

func WithCidrList(cidrList []netip.Prefix) Option {
	return func(i *IPScanner) {
		i.options.CidrList = cidrList
	}
}

func WithHTTPPing() Option {
	return func(i *IPScanner) {
		i.options.SelectedOps |= statute.HTTPPing
	}
}

func WithWarpPing() Option {
	return func(i *IPScanner) {
		i.options.SelectedOps |= statute.WARPPing
	}
}

func WithQUICPing() Option {
	return func(i *IPScanner) {
		i.options.SelectedOps |= statute.QUICPing
	}
}

func WithTCPPing() Option {
	return func(i *IPScanner) {
		i.options.SelectedOps |= statute.TCPPing
	}
}

func WithTLSPing() Option {
	return func(i *IPScanner) {
		i.options.SelectedOps |= statute.TLSPing
	}
}

func WithIPQueueSize(size int) Option {
	return func(i *IPScanner) {
		i.options.IPQueueSize = size
	}
}

func WithMaxDesirableRTT(threshold time.Duration) Option {
	return func(i *IPScanner) {
		i.options.MaxDesirableRTT = threshold
	}
}

func WithIPQueueTTL(ttl time.Duration) Option {
	return func(i *IPScanner) {
		i.options.IPQueueTTL = ttl
	}
}

func WithConnectionTimeout(timeout time.Duration) Option {
	return func(i *IPScanner) {
		i.options.ConnectionTimeout = timeout
	}
}

func WithHandshakeTimeout(timeout time.Duration) Option {
	return func(i *IPScanner) {
		i.options.HandshakeTimeout = timeout
	}
}

func WithTlsVersion(version uint16) Option {
	return func(i *IPScanner) {
		i.options.TlsVersion = version
	}
}

func WithWarpPrivateKey(privateKey string) Option {
	return func(i *IPScanner) {
		i.options.WarpPrivateKey = privateKey
	}
}

func WithWarpPeerPublicKey(peerPublicKey string) Option {
	return func(i *IPScanner) {
		i.options.WarpPeerPublicKey = peerPublicKey
	}
}

func WithWarpPreSharedKey(presharedKey string) Option {
	return func(i *IPScanner) {
		i.options.WarpPresharedKey = presharedKey
	}
}

// run engine and in case of new event call onChange callback also if it gets canceled with context
// cancel all operations

func (i *IPScanner) Run(ctx context.Context) {
	statute.FinalOptions = &i.options
	if !i.options.UseIPv4 && !i.options.UseIPv6 {
		i.log.Error("Fatal: both IPv4 and IPv6 are disabled, nothing to do")
		return
	}
	i.engine = engine.NewScannerEngine(&i.options)
	go i.engine.Run(ctx)
}

func (i *IPScanner) GetAvailableIPs() []statute.IPInfo {
	if i.engine != nil {
		return i.engine.GetAvailableIPs(false)
	}
	return nil
}

func CanConnectIPv6(remoteAddr netip.AddrPort) bool {
	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.Dial("tcp6", remoteAddr.String())
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

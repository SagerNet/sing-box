//go:build linux

package inbound

import (
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"

	"github.com/sagernet/nftables"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/redir"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

type tunAutoRedirect struct {
	myInboundAdapter
	tunOptions    *tun.Options
	enableIPv4    bool
	enableIPv6    bool
	iptablesPath  string
	ip6tablesPath string
	useNfTables   bool
	androidSu     bool
	suPath        string
}

func newAutoRedirect(t *Tun) (*tunAutoRedirect, error) {
	s := &tunAutoRedirect{
		myInboundAdapter: myInboundAdapter{
			protocol: C.TypeRedirect,
			network:  []string{N.NetworkTCP},
			ctx:      t.ctx,
			router:   t.router,
			logger:   t.logger,
			tag:      t.tag,
			listenOptions: option.ListenOptions{
				InboundOptions: t.inboundOptions,
			},
		},
		tunOptions: &t.tunOptions,
	}
	s.connHandler = s

	if C.IsAndroid {
		s.enableIPv4 = true
		s.iptablesPath = "/system/bin/iptables"
		userId := os.Getuid()
		if userId != 0 {
			var (
				suPath string
				err    error
			)
			if t.platformInterface != nil {
				suPaths := []string{
					"/bin/su",
					"/system/bin/su",
				}
				for _, path := range suPaths {
					suPath, err = exec.LookPath(path)
					if err == nil {
						break
					}
				}
			} else {
				suPath, err = exec.LookPath("su")
			}
			if err == nil {
				s.androidSu = true
				s.suPath = suPath
			} else {
				return nil, E.Extend(E.Cause(err, "root permission is required for auto redirect"), os.Getenv("PATH"))
			}
		}
	} else {
		err := s.initializeNfTables()
		if err != nil && err != os.ErrInvalid {
			t.logger.Debug("device has no nftables support: ", err)
		}
		if len(t.tunOptions.Inet4Address) > 0 {
			s.enableIPv4 = true
			if !s.useNfTables {
				s.iptablesPath, err = exec.LookPath("iptables")
				if err != nil {
					return nil, E.Cause(err, "iptables is required")
				}
			}
		}
		if len(t.tunOptions.Inet6Address) > 0 {
			s.enableIPv6 = true
			if !s.useNfTables {
				s.ip6tablesPath, err = exec.LookPath("ip6tables")
				if err != nil {
					if !s.enableIPv4 {
						return nil, E.Cause(err, "ip6tables is required")
					} else {
						s.enableIPv6 = false
						t.logger.Error("device has no ip6tables nat support: ", err)
					}
				}
			}
		}
	}
	var listenAddr netip.Addr
	if C.IsAndroid {
		listenAddr = netip.AddrFrom4([4]byte{127, 0, 0, 1})
	} else if s.enableIPv6 {
		listenAddr = netip.IPv6Unspecified()
	} else {
		listenAddr = netip.IPv4Unspecified()
	}
	s.listenOptions.Listen = option.NewListenAddress(listenAddr)
	return s, nil
}

func (t *tunAutoRedirect) initializeNfTables() error {
	disabled, err := strconv.ParseBool(os.Getenv("AUTO_REDIRECT_DISABLE_NFTABLES"))
	if err == nil && disabled {
		return os.ErrInvalid
	}
	nft, err := nftables.New()
	if err != nil {
		return err
	}
	defer nft.CloseLasting()
	_, err = nft.ListTablesOfFamily(unix.AF_INET)
	if err != nil {
		return err
	}
	t.useNfTables = true
	return nil
}

func (t *tunAutoRedirect) Start() error {
	err := t.myInboundAdapter.Start()
	if err != nil {
		return E.Cause(err, "start redirect server")
	}
	t.cleanupTables()
	err = t.setupTables()
	if err != nil {
		return err
	}
	return nil
}

func (t *tunAutoRedirect) Close() error {
	t.cleanupTables()
	return t.myInboundAdapter.Close()
}

func (t *tunAutoRedirect) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	destination, err := redir.GetOriginalDestination(conn)
	if err != nil {
		return E.Cause(err, "get redirect destination")
	}
	metadata.Destination = M.SocksaddrFromNetIP(destination)
	return t.newConnection(ctx, conn, metadata)
}

func (t *tunAutoRedirect) setupTables() error {
	var setupTables func(int) error
	if t.useNfTables {
		setupTables = t.setupNfTables
	} else {
		setupTables = t.setupIPTables
	}
	if t.enableIPv4 {
		err := setupTables(unix.AF_INET)
		if err != nil {
			return err
		}
	}
	if t.enableIPv6 {
		err := setupTables(unix.AF_INET6)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *tunAutoRedirect) cleanupTables() {
	var cleanupTables func(int)
	if t.useNfTables {
		cleanupTables = t.cleanupNfTables
	} else {
		cleanupTables = t.cleanupIPTables
	}
	if t.enableIPv4 {
		cleanupTables(unix.AF_INET)
	}
	if t.enableIPv6 {
		cleanupTables(unix.AF_INET6)
	}
}

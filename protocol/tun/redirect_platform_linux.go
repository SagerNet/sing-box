//go:build linux

package tun

import (
	"net/netip"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

type platformAutoRedirect struct {
	inbound        *Inbound
	redirectServer *tun.RedirectServer
	sessionAccess  sync.Mutex
	session        adapter.AutoRedirectSession
}

func newPlatformAutoRedirect(inbound *Inbound) (tun.AutoRedirect, error) {
	return &platformAutoRedirect{inbound: inbound}, nil
}

func (r *platformAutoRedirect) Start() error {
	handler := (*autoRedirectHandler)(r.inbound)
	listenAddress := netip.IPv4Unspecified()
	if len(r.inbound.tunOptions.Inet6Address) > 0 {
		listenAddress = netip.IPv6Unspecified()
	}
	redirectServer := tun.NewRedirectServer(r.inbound.ctx, handler, r.inbound.logger, listenAddress)
	if listenAddress.Is6() {
		redirectServer.SetExternalTransparent()
	}
	err := redirectServer.Start()
	if err != nil {
		return E.Cause(err, "start redirect server")
	}
	session, err := r.inbound.platformInterface.CreateAutoRedirect(adapter.AutoRedirectOptions{
		TunOptions:                     &r.inbound.tunOptions,
		TableName:                      "sing-box",
		RedirectPort:                   redirectServer.Port(),
		RedirectListenerFileDescriptor: redirectServer.ListenerFileDescriptor,
		RouteAddressSetFileDescriptor:  r.routeAddressSetFileDescriptor,
		Handler:                        handler,
	})
	if err != nil {
		_ = redirectServer.Close()
		return err
	}
	r.redirectServer = redirectServer
	r.sessionAccess.Lock()
	r.session = session
	r.sessionAccess.Unlock()
	return nil
}

func (r *platformAutoRedirect) routeAddressSetFileDescriptor() (int, error) {
	include, exclude := r.inbound.routeAddressSetPrefixes()
	var pipeFds [2]int
	err := unix.Pipe2(pipeFds[:], unix.O_CLOEXEC)
	if err != nil {
		return 0, E.Cause(err, "create route address set pipe")
	}
	writer := os.NewFile(uintptr(pipeFds[1]), "route-address-set")
	go func() {
		writeErr := srs.WriteRouteAddressSet(writer, include, exclude)
		if writeErr != nil {
			r.inbound.logger.Error("write route address set: ", writeErr)
		}
		_ = writer.Close()
	}()
	return pipeFds[0], nil
}

func (r *platformAutoRedirect) UpdateRouteAddressSet() error {
	r.sessionAccess.Lock()
	session := r.session
	r.sessionAccess.Unlock()
	if session == nil {
		return nil
	}
	return session.UpdateRouteAddressSet()
}

func (r *platformAutoRedirect) Close() error {
	r.sessionAccess.Lock()
	session := r.session
	r.sessionAccess.Unlock()
	return common.Close(session, common.PtrOrNil(r.redirectServer))
}

package dialer

import (
	"context"
	"net"
	"net/netip"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (d *DefaultDialer) dialParallelInterface(ctx context.Context, dialer net.Dialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, bool, error) {
	primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
	if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
		return nil, false, E.New("no available network interface")
	}
	defaultInterface := d.networkManager.InterfaceMonitor().DefaultInterface()
	if len(primaryInterfaces)+len(fallbackInterfaces) == 1 {
		var (
			iif     adapter.NetworkInterface
			primary bool
		)
		if len(primaryInterfaces) == 1 {
			iif = primaryInterfaces[0]
			primary = true
		} else {
			iif = fallbackInterfaces[0]
		}
		conn, err := d.dialInterface(ctx, dialer, network, destination, destinationAddresses, iif, defaultInterface, fallbackDelay)
		if err != nil {
			return nil, false, err
		}
		return conn, primary, nil
	}
	if fallbackDelay == 0 {
		fallbackDelay = N.DefaultFallbackDelay
	}
	returned := make(chan struct{})
	defer close(returned)
	type dialResult struct {
		net.Conn
		error
		primary bool
	}
	results := make(chan dialResult) // unbuffered
	startRacer := func(ctx context.Context, primary bool, iif adapter.NetworkInterface) {
		conn, err := d.dialInterface(ctx, dialer, network, destination, destinationAddresses, iif, defaultInterface, fallbackDelay)
		if err != nil {
			select {
			case results <- dialResult{error: err, primary: primary}:
			case <-returned:
			}
		} else {
			select {
			case results <- dialResult{Conn: conn, primary: primary}:
			case <-returned:
				conn.Close()
			}
		}
	}
	primaryCtx, primaryCancel := context.WithCancel(ctx)
	defer primaryCancel()
	for _, iif := range primaryInterfaces {
		go startRacer(primaryCtx, true, iif)
	}
	var (
		fallbackTimer *time.Timer
		fallbackChan  <-chan time.Time
	)
	if len(fallbackInterfaces) > 0 {
		fallbackTimer = time.NewTimer(fallbackDelay)
		defer fallbackTimer.Stop()
		fallbackChan = fallbackTimer.C
	}
	var errors []error
	for {
		select {
		case <-fallbackChan:
			fallbackCtx, fallbackCancel := context.WithCancel(ctx)
			defer fallbackCancel()
			for _, iif := range fallbackInterfaces {
				go startRacer(fallbackCtx, false, iif)
			}
		case res := <-results:
			if res.error == nil {
				return res.Conn, res.primary, nil
			}
			errors = append(errors, res.error)
			if len(errors) == len(primaryInterfaces)+len(fallbackInterfaces) {
				return nil, false, E.Errors(errors...)
			}
			if res.primary && fallbackTimer != nil && fallbackTimer.Stop() {
				fallbackTimer.Reset(0)
			}
		}
	}
}

func (d *DefaultDialer) dialParallelInterfaceFastFallback(ctx context.Context, dialer net.Dialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration, resetFastFallback func(time.Time)) (net.Conn, bool, error) {
	primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
	if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
		return nil, false, E.New("no available network interface")
	}
	defaultInterface := d.networkManager.InterfaceMonitor().DefaultInterface()
	if len(primaryInterfaces)+len(fallbackInterfaces) == 1 {
		var (
			iif     adapter.NetworkInterface
			primary bool
		)
		if len(primaryInterfaces) == 1 {
			iif = primaryInterfaces[0]
			primary = true
		} else {
			iif = fallbackInterfaces[0]
		}
		conn, err := d.dialInterface(ctx, dialer, network, destination, destinationAddresses, iif, defaultInterface, fallbackDelay)
		if err != nil {
			return nil, false, err
		}
		return conn, primary, nil
	}
	if fallbackDelay == 0 {
		fallbackDelay = N.DefaultFallbackDelay
	}
	returned := make(chan struct{})
	defer close(returned)
	type dialResult struct {
		net.Conn
		error
		primary bool
	}
	startAt := time.Now()
	results := make(chan dialResult) // unbuffered
	startRacer := func(ctx context.Context, primary bool, iif adapter.NetworkInterface) {
		conn, err := d.dialInterface(ctx, dialer, network, destination, destinationAddresses, iif, defaultInterface, fallbackDelay)
		if err != nil {
			select {
			case results <- dialResult{error: err, primary: primary}:
			case <-returned:
			}
		} else {
			select {
			case results <- dialResult{Conn: conn, primary: primary}:
			case <-returned:
				if primary && time.Since(startAt) <= fallbackDelay {
					resetFastFallback(time.Time{})
				}
				conn.Close()
			}
		}
	}
	for _, iif := range primaryInterfaces {
		go startRacer(ctx, true, iif)
	}
	fallbackCtx, fallbackCancel := context.WithCancel(ctx)
	defer fallbackCancel()
	for _, iif := range fallbackInterfaces {
		go startRacer(fallbackCtx, false, iif)
	}
	var errors []error
	for res := range results {
		if res.error == nil {
			return res.Conn, res.primary, nil
		}
		errors = append(errors, res.error)
		if len(errors) == len(primaryInterfaces)+len(fallbackInterfaces) {
			return nil, false, E.Errors(errors...)
		}
	}
	return nil, false, E.Errors(errors...)
}

func (d *DefaultDialer) dialInterface(ctx context.Context, dialer net.Dialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, iif adapter.NetworkInterface, defaultInterface *control.Interface, fallbackDelay time.Duration) (net.Conn, error) {
	if defaultInterface == nil || iif.Index != defaultInterface.Index {
		dialer.Control = control.Append(dialer.Control, control.BindToInterface(nil, iif.Name, iif.Index))
	}
	boundDialer := &interfaceDialer{
		parent:         d,
		dialer:         dialer,
		interfaceIndex: iif.Index,
	}
	var (
		conn net.Conn
		err  error
	)
	if d.tcpFastOpen {
		conn, err = N.DialSerial(ctx, boundDialer, network, destination, destinationAddresses)
	} else {
		conn, err = N.DialParallel(ctx, boundDialer, network, destination, destinationAddresses, destinationAddresses[0].Is6(), fallbackDelay)
	}
	if err != nil {
		return nil, E.Cause(err, "dial ", iif.Name, " (", iif.Index, ")")
	}
	return conn, nil
}

type interfaceDialer struct {
	parent         *DefaultDialer
	dialer         net.Dialer
	interfaceIndex int
}

func (d *interfaceDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if destination.Addr.Zone() == "" && isLinkScoped(destination.Addr) {
		destination.Addr = destination.Addr.WithZone(strconv.Itoa(d.interfaceIndex))
	}
	conn, err := d.dialer.DialContext(ctx, network, destination.String())
	return d.parent.trackConn(ctx, destination, conn, err)
}

func (d *interfaceDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (d *DefaultDialer) serialInterfaceControl(strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType) control.Func {
	return func(network, address string, conn syscall.RawConn) error {
		primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
		if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
			return E.New("no available network interface")
		}
		defaultInterface := d.networkManager.InterfaceMonitor().DefaultInterface()
		var errors []error
		for _, networkInterfaces := range [][]adapter.NetworkInterface{primaryInterfaces, fallbackInterfaces} {
			for _, networkInterface := range networkInterfaces {
				if defaultInterface != nil && networkInterface.Index == defaultInterface.Index {
					return nil
				}
				err := control.BindToInterface0(nil, conn, network, address, networkInterface.Name, networkInterface.Index, false)
				if err == nil {
					return nil
				}
				errors = append(errors, E.Cause(err, "bind ", networkInterface.Name, " (", networkInterface.Index, ")"))
			}
		}
		return E.Errors(errors...)
	}
}

func selectInterfaces(networkManager adapter.NetworkManager, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType) (primaryInterfaces []adapter.NetworkInterface, fallbackInterfaces []adapter.NetworkInterface) {
	interfaces := networkManager.NetworkInterfaces()
	myInterfaces := networkManager.InterfaceMonitor().MyInterfaces()
	if len(myInterfaces) > 0 {
		interfaces = common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
			return !common.Contains(myInterfaces, it.Name)
		})
	}
	switch strategy {
	case C.NetworkStrategyDefault:
		if len(interfaceType) == 0 {
			defaultIf := networkManager.InterfaceMonitor().DefaultInterface()
			if defaultIf != nil {
				for _, iif := range interfaces {
					if iif.Index == defaultIf.Index {
						primaryInterfaces = append(primaryInterfaces, iif)
					}
				}
			} else {
				primaryInterfaces = interfaces
			}
		} else {
			primaryInterfaces = common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, it.Type)
			})
		}
	case C.NetworkStrategyHybrid:
		if len(interfaceType) == 0 {
			primaryInterfaces = interfaces
		} else {
			primaryInterfaces = common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, it.Type)
			})
		}
	case C.NetworkStrategyFallback:
		if len(interfaceType) == 0 {
			defaultIf := networkManager.InterfaceMonitor().DefaultInterface()
			if defaultIf != nil {
				for _, iif := range interfaces {
					if iif.Index == defaultIf.Index {
						primaryInterfaces = append(primaryInterfaces, iif)
						break
					}
				}
			} else {
				primaryInterfaces = interfaces
			}
		} else {
			primaryInterfaces = common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, it.Type)
			})
		}
		if len(fallbackInterfaceType) == 0 {
			fallbackInterfaces = common.Filter(interfaces, func(it adapter.NetworkInterface) bool {
				return !common.Any(primaryInterfaces, func(iif adapter.NetworkInterface) bool {
					return it.Index == iif.Index
				})
			})
		} else {
			fallbackInterfaces = common.Filter(interfaces, func(iif adapter.NetworkInterface) bool {
				return common.Contains(fallbackInterfaceType, iif.Type)
			})
		}
	}
	return primaryInterfaces, fallbackInterfaces
}

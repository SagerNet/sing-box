package dialer

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

func (d *DefaultDialer) dialParallelInterface(ctx context.Context, dialer net.Dialer, network string, addr string, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, bool, error) {
	primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
	if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
		return nil, false, E.New("no available network interface")
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
		perNetDialer := dialer
		perNetDialer.Control = control.Append(perNetDialer.Control, control.BindToInterface(nil, iif.Name, iif.Index))
		conn, err := perNetDialer.DialContext(ctx, network, addr)
		if err != nil {
			select {
			case results <- dialResult{error: E.Cause(err, "dial ", iif.Name, " (", iif.Name, ")"), primary: primary}:
			case <-returned:
			}
		} else {
			select {
			case results <- dialResult{Conn: conn}:
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

func (d *DefaultDialer) dialParallelInterfaceFastFallback(ctx context.Context, dialer net.Dialer, network string, addr string, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration, resetFastFallback func(time.Time)) (net.Conn, bool, error) {
	primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
	if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
		return nil, false, E.New("no available network interface")
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
		perNetDialer := dialer
		perNetDialer.Control = control.Append(perNetDialer.Control, control.BindToInterface(nil, iif.Name, iif.Index))
		conn, err := perNetDialer.DialContext(ctx, network, addr)
		if err != nil {
			select {
			case results <- dialResult{error: E.Cause(err, "dial ", iif.Name, " (", iif.Name, ")"), primary: primary}:
			case <-returned:
			}
		} else {
			select {
			case results <- dialResult{Conn: conn}:
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
	for {
		select {
		case res := <-results:
			if res.error == nil {
				return res.Conn, res.primary, nil
			}
			errors = append(errors, res.error)
			if len(errors) == len(primaryInterfaces)+len(fallbackInterfaces) {
				return nil, false, E.Errors(errors...)
			}
		}
	}
}

func (d *DefaultDialer) listenSerialInterfacePacket(ctx context.Context, listener net.ListenConfig, network string, addr string, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, error) {
	primaryInterfaces, fallbackInterfaces := selectInterfaces(d.networkManager, strategy, interfaceType, fallbackInterfaceType)
	if len(primaryInterfaces)+len(fallbackInterfaces) == 0 {
		return nil, E.New("no available network interface")
	}
	if fallbackDelay == 0 {
		fallbackDelay = N.DefaultFallbackDelay
	}
	var errors []error
	for _, primaryInterface := range primaryInterfaces {
		perNetListener := listener
		perNetListener.Control = control.Append(perNetListener.Control, control.BindToInterface(nil, primaryInterface.Name, primaryInterface.Index))
		conn, err := perNetListener.ListenPacket(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		errors = append(errors, E.Cause(err, "listen ", primaryInterface.Name, " (", primaryInterface.Name, ")"))
	}
	for _, fallbackInterface := range fallbackInterfaces {
		perNetListener := listener
		perNetListener.Control = control.Append(perNetListener.Control, control.BindToInterface(nil, fallbackInterface.Name, fallbackInterface.Index))
		conn, err := perNetListener.ListenPacket(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		errors = append(errors, E.Cause(err, "listen ", fallbackInterface.Name, " (", fallbackInterface.Name, ")"))
	}
	return nil, E.Errors(errors...)
}

func selectInterfaces(networkManager adapter.NetworkManager, strategy C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType) (primaryInterfaces []adapter.NetworkInterface, fallbackInterfaces []adapter.NetworkInterface) {
	interfaces := networkManager.NetworkInterfaces()
	switch strategy {
	case C.NetworkStrategyDefault:
		if len(interfaceType) == 0 {
			defaultIf := networkManager.InterfaceMonitor().DefaultInterface()
			for _, iif := range interfaces {
				if iif.Index == defaultIf.Index {
					primaryInterfaces = append(primaryInterfaces, iif)
				} else {
					fallbackInterfaces = append(fallbackInterfaces, iif)
				}
			}
		} else {
			primaryInterfaces = common.Filter(interfaces, func(iif adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, iif.Type)
			})
		}
	case C.NetworkStrategyHybrid:
		if len(interfaceType) == 0 {
			primaryInterfaces = interfaces
		} else {
			primaryInterfaces = common.Filter(interfaces, func(iif adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, iif.Type)
			})
		}
	case C.NetworkStrategyFallback:
		if len(interfaceType) == 0 {
			defaultIf := networkManager.InterfaceMonitor().DefaultInterface()
			for _, iif := range interfaces {
				if iif.Index == defaultIf.Index {
					primaryInterfaces = append(primaryInterfaces, iif)
				} else {
					fallbackInterfaces = append(fallbackInterfaces, iif)
				}
			}
		} else {
			primaryInterfaces = common.Filter(interfaces, func(iif adapter.NetworkInterface) bool {
				return common.Contains(interfaceType, iif.Type)
			})
		}
		fallbackInterfaces = common.Filter(interfaces, func(iif adapter.NetworkInterface) bool {
			return common.Contains(fallbackInterfaceType, iif.Type)
		})
	}
	return primaryInterfaces, fallbackInterfaces
}

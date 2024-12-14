package dialer

import (
	"context"
	"net"
	"net/netip"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func DialSerialNetwork(ctx context.Context, dialer N.Dialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if len(destinationAddresses) == 0 {
		if !destination.IsIP() {
			panic("invalid usage")
		}
		destinationAddresses = []netip.Addr{destination.Addr}
	}
	if parallelDialer, isParallel := dialer.(ParallelNetworkDialer); isParallel {
		return parallelDialer.DialParallelNetwork(ctx, network, destination, destinationAddresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	}
	var errors []error
	if parallelDialer, isParallel := dialer.(ParallelInterfaceDialer); isParallel {
		for _, address := range destinationAddresses {
			conn, err := parallelDialer.DialParallelInterface(ctx, network, M.SocksaddrFrom(address, destination.Port), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
			if err == nil {
				return conn, nil
			}
			errors = append(errors, err)
		}
	} else {
		for _, address := range destinationAddresses {
			conn, err := dialer.DialContext(ctx, network, M.SocksaddrFrom(address, destination.Port))
			if err == nil {
				return conn, nil
			}
			errors = append(errors, err)
		}
	}
	return nil, E.Errors(errors...)
}

func DialParallelNetwork(ctx context.Context, dialer ParallelInterfaceDialer, network string, destination M.Socksaddr, destinationAddresses []netip.Addr, preferIPv6 bool, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.Conn, error) {
	if len(destinationAddresses) == 0 {
		if !destination.IsIP() {
			panic("invalid usage")
		}
		destinationAddresses = []netip.Addr{destination.Addr}
	}

	if fallbackDelay == 0 {
		fallbackDelay = N.DefaultFallbackDelay
	}

	returned := make(chan struct{})
	defer close(returned)

	addresses4 := common.Filter(destinationAddresses, func(address netip.Addr) bool {
		return address.Is4() || address.Is4In6()
	})
	addresses6 := common.Filter(destinationAddresses, func(address netip.Addr) bool {
		return address.Is6() && !address.Is4In6()
	})
	if len(addresses4) == 0 || len(addresses6) == 0 {
		return DialSerialNetwork(ctx, dialer, network, destination, destinationAddresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	}
	var primaries, fallbacks []netip.Addr
	if preferIPv6 {
		primaries = addresses6
		fallbacks = addresses4
	} else {
		primaries = addresses4
		fallbacks = addresses6
	}
	type dialResult struct {
		net.Conn
		error
		primary bool
		done    bool
	}
	results := make(chan dialResult) // unbuffered
	startRacer := func(ctx context.Context, primary bool) {
		ras := primaries
		if !primary {
			ras = fallbacks
		}
		c, err := DialSerialNetwork(ctx, dialer, network, destination, ras, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
		select {
		case results <- dialResult{Conn: c, error: err, primary: primary, done: true}:
		case <-returned:
			if c != nil {
				c.Close()
			}
		}
	}
	var primary, fallback dialResult
	primaryCtx, primaryCancel := context.WithCancel(ctx)
	defer primaryCancel()
	go startRacer(primaryCtx, true)
	fallbackTimer := time.NewTimer(fallbackDelay)
	defer fallbackTimer.Stop()
	for {
		select {
		case <-fallbackTimer.C:
			fallbackCtx, fallbackCancel := context.WithCancel(ctx)
			defer fallbackCancel()
			go startRacer(fallbackCtx, false)

		case res := <-results:
			if res.error == nil {
				return res.Conn, nil
			}
			if res.primary {
				primary = res
			} else {
				fallback = res
			}
			if primary.done && fallback.done {
				return nil, primary.error
			}
			if res.primary && fallbackTimer.Stop() {
				fallbackTimer.Reset(0)
			}
		}
	}
}

func ListenSerialNetworkPacket(ctx context.Context, dialer N.Dialer, destination M.Socksaddr, destinationAddresses []netip.Addr, strategy *C.NetworkStrategy, interfaceType []C.InterfaceType, fallbackInterfaceType []C.InterfaceType, fallbackDelay time.Duration) (net.PacketConn, netip.Addr, error) {
	if len(destinationAddresses) == 0 {
		if !destination.IsIP() {
			panic("invalid usage")
		}
		destinationAddresses = []netip.Addr{destination.Addr}
	}
	if parallelDialer, isParallel := dialer.(ParallelNetworkDialer); isParallel {
		return parallelDialer.ListenSerialNetworkPacket(ctx, destination, destinationAddresses, strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
	}
	var errors []error
	if parallelDialer, isParallel := dialer.(ParallelInterfaceDialer); isParallel {
		for _, address := range destinationAddresses {
			conn, err := parallelDialer.ListenSerialInterfacePacket(ctx, M.SocksaddrFrom(address, destination.Port), strategy, interfaceType, fallbackInterfaceType, fallbackDelay)
			if err == nil {
				return conn, address, nil
			}
			errors = append(errors, err)
		}
	} else {
		for _, address := range destinationAddresses {
			conn, err := dialer.ListenPacket(ctx, M.SocksaddrFrom(address, destination.Port))
			if err == nil {
				return conn, address, nil
			}
			errors = append(errors, err)
		}
	}
	return nil, netip.Addr{}, E.Errors(errors...)
}

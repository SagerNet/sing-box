// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

package local

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"unsafe"
	_ "unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	mDNS "github.com/miekg/dns"
)

type (
	_C_char               = byte
	_C_int                = int32
	_C_uchar              = byte
	_C_ushort             = uint16
	_C_uint               = uint32
	_C_ulong              = uint64
	_C_struct___res_state = ResState
	_C_struct_sockaddr    = syscall.RawSockaddr
)

func _C_free(p unsafe.Pointer) { runtime.KeepAlive(p) }

func _C_malloc(n uintptr) unsafe.Pointer {
	if n <= 0 {
		n = 1
	}
	return unsafe.Pointer(&make([]byte, n)[0])
}

const (
	MAXNS     = 3
	MAXDNSRCH = 6
)

type ResState struct {
	Retrans    _C_int
	Retry      _C_int
	Options    _C_ulong
	Nscount    _C_int
	Nsaddrlist [MAXNS]_C_struct_sockaddr
	Id         _C_ushort
	Dnsrch     [MAXDNSRCH + 1]*_C_char
	Defname    [256]_C_char
	Pfcode     _C_ulong
	Ndots      _C_uint
	Nsort      _C_uint
	stub       [128]byte
}

//go:linkname ResNinit internal/syscall/unix.ResNinit
func ResNinit(state *_C_struct___res_state) error

//go:linkname ResNsearch internal/syscall/unix.ResNsearch
func ResNsearch(state *_C_struct___res_state, dname *byte, class, typ int, ans *byte, anslen int) (int, error)

//go:linkname ResNclose internal/syscall/unix.ResNclose
func ResNclose(state *_C_struct___res_state)

//go:linkname GoString internal/syscall/unix.GoString
func GoString(p *byte) string

// doBlockingWithCtx executes a blocking function in a separate goroutine when the provided
// context is cancellable. It is intended for use with calls that don't support context
// cancellation (cgo, syscalls). blocking func may still be running after this function finishes.
// For the duration of the execution of the blocking function, the thread is 'acquired' using [acquireThread],
// blocking might not be executed when the context gets canceled early.
func doBlockingWithCtx[T any](ctx context.Context, blocking func() (T, error)) (T, error) {
	if err := acquireThread(ctx); err != nil {
		var zero T
		return zero, err
	}

	if ctx.Done() == nil {
		defer releaseThread()
		return blocking()
	}

	type result struct {
		res T
		err error
	}

	res := make(chan result, 1)
	go func() {
		defer releaseThread()
		var r result
		r.res, r.err = blocking()
		res <- r
	}()

	select {
	case r := <-res:
		return r.res, r.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

//go:linkname acquireThread net.acquireThread
func acquireThread(ctx context.Context) error

//go:linkname releaseThread net.releaseThread
func releaseThread()

func cgoResSearch(hostname string, rtype, class int) (*mDNS.Msg, error) {
	resStateSize := unsafe.Sizeof(_C_struct___res_state{})
	var state *_C_struct___res_state
	if resStateSize > 0 {
		mem := _C_malloc(resStateSize)
		defer _C_free(mem)
		memSlice := unsafe.Slice((*byte)(mem), resStateSize)
		clear(memSlice)
		state = (*_C_struct___res_state)(unsafe.Pointer(&memSlice[0]))
	}
	if err := ResNinit(state); err != nil {
		return nil, errors.New("res_ninit failure: " + err.Error())
	}
	defer ResNclose(state)

	bufSize := maxDNSPacketSize
	buf := (*_C_uchar)(_C_malloc(uintptr(bufSize)))
	defer _C_free(unsafe.Pointer(buf))

	s, err := syscall.BytePtrFromString(hostname)
	if err != nil {
		return nil, err
	}

	var size int
	for {
		size, _ = ResNsearch(state, s, class, rtype, buf, bufSize)
		if size <= bufSize || size > 0xffff {
			break
		}

		// Allocate a bigger buffer to fit the entire msg.
		_C_free(unsafe.Pointer(buf))
		bufSize = size
		buf = (*_C_uchar)(_C_malloc(uintptr(bufSize)))
	}

	var msg mDNS.Msg
	if size == -1 {
		// macOS's libresolv seems to directly return -1 for responses that are not success responses but are exchanged.
		// However, we still need the response, so we fall back to parsing the entire buffer.
		err = msg.Unpack(unsafe.Slice(buf, bufSize))
		if err != nil {
			return nil, E.New("res_nsearch failure")
		}
	} else {
		err = msg.Unpack(unsafe.Slice(buf, size))
		if err != nil {
			return nil, err
		}
	}
	return &msg, nil
}

//go:build windows

package winmutex

import (
	"errors"
	"runtime"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/windows"
)

func WithLock[T any](name string, timeout time.Duration, operation func() (T, error)) (result T, err error) {
	if timeout < 0 || timeout/time.Millisecond >= time.Duration(windows.INFINITE) {
		return result, E.New("invalid named mutex timeout: ", timeout)
	}
	namePointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return result, E.Cause(err, "encode named mutex ", name)
	}
	runtime.LockOSThread()
	handle, err := windows.CreateMutex(nil, false, namePointer)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		runtime.UnlockOSThread()
		return result, E.Cause(err, "create named mutex ", name)
	}
	waitMilliseconds := uint32((timeout + time.Millisecond - 1) / time.Millisecond)
	waitResult, err := windows.WaitForSingleObject(handle, waitMilliseconds)
	if err != nil {
		closeErr := windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		if closeErr != nil {
			closeErr = E.Cause(closeErr, "close named mutex ", name)
		}
		return result, E.Errors(E.Cause(err, "wait named mutex ", name), closeErr)
	}
	switch waitResult {
	case uint32(windows.WAIT_OBJECT_0), uint32(windows.WAIT_ABANDONED):
	case uint32(windows.WAIT_TIMEOUT):
		closeErr := windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		if closeErr != nil {
			return result, E.Errors(
				E.New("wait named mutex ", name, ": timeout after ", timeout),
				E.Cause(closeErr, "close named mutex ", name),
			)
		}
		return result, E.New("wait named mutex ", name, ": timeout after ", timeout)
	default:
		closeErr := windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		unexpectedErr := E.New("wait named mutex ", name, ": unexpected result ", waitResult)
		if closeErr != nil {
			return result, E.Errors(unexpectedErr, E.Cause(closeErr, "close named mutex ", name))
		}
		return result, unexpectedErr
	}
	defer runtime.UnlockOSThread()
	defer func() {
		releaseErr := windows.ReleaseMutex(handle)
		if releaseErr != nil {
			releaseErr = E.Cause(releaseErr, "release named mutex ", name)
		}
		closeErr := windows.CloseHandle(handle)
		if closeErr != nil {
			closeErr = E.Cause(closeErr, "close named mutex ", name)
		}
		err = E.Errors(err, releaseErr, closeErr)
	}()
	return operation()
}

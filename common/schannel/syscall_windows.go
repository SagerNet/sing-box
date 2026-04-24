package schannel

import (
	"syscall"
	"unsafe"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go

// secur32.dll — SSPI / Schannel interface

//sys sspiAcquireCredentialsHandle(principal *uint16, pkgname *uint16, credentialUse uint32, logonID *uint64, authData unsafe.Pointer, getKeyFn uintptr, getKeyArg uintptr, credential *secHandle, expiry *windows.Filetime) (ret syscall.Errno) = secur32.AcquireCredentialsHandleW
//sys sspiFreeCredentialsHandle(credential *secHandle) (ret syscall.Errno) = secur32.FreeCredentialsHandle
//sys sspiInitializeSecurityContext(credential *secHandle, context *secHandle, targetName *uint16, contextReq uint32, reserved1 uint32, targetDataRep uint32, input *secBufferDesc, reserved2 uint32, newContext *secHandle, output *secBufferDesc, contextAttr *uint32, expiry *windows.Filetime) (ret syscall.Errno) = secur32.InitializeSecurityContextW
//sys sspiDeleteSecurityContext(context *secHandle) (ret syscall.Errno) = secur32.DeleteSecurityContext
//sys sspiQueryContextAttributes(context *secHandle, attribute uint32, buffer unsafe.Pointer) (ret syscall.Errno) = secur32.QueryContextAttributesW
//sys sspiEncryptMessage(context *secHandle, qop uint32, message *secBufferDesc, sequenceNumber uint32) (ret syscall.Errno) = secur32.EncryptMessage
//sys sspiDecryptMessage(context *secHandle, message *secBufferDesc, sequenceNumber uint32, qop *uint32) (ret syscall.Errno) = secur32.DecryptMessage
//sys sspiFreeContextBuffer(buffer *byte) (ret syscall.Errno) = secur32.FreeContextBuffer

// mkwinsyscall does not emit CompleteAuthToken for this package, so bind it manually.
var procCompleteAuthToken = modsecur32.NewProc("CompleteAuthToken")

func sspiCompleteAuthToken(context *secHandle, token *secBufferDesc) (ret syscall.Errno) {
	r0, _, _ := syscall.SyscallN(procCompleteAuthToken.Addr(), uintptr(unsafe.Pointer(context)), uintptr(unsafe.Pointer(token)))
	ret = syscall.Errno(r0)
	return
}

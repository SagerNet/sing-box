//go:build windows

package winhttp

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go

//sys winHTTPOpen(userAgent *uint16, accessType uint32, proxy *uint16, proxyBypass *uint16, flags uint32) (sessionHandle Handle, err error) = winhttp.WinHttpOpen
//sys winHTTPSetStatusCallback(handle Handle, callback uintptr, notificationFlags uint32, reserved uintptr) (previousCallback uintptr, err error) [failretval==invalidStatusCallback] = winhttp.WinHttpSetStatusCallback
//sys winHTTPCloseHandle(handle Handle) (err error) = winhttp.WinHttpCloseHandle
//sys winHTTPConnect(sessionHandle Handle, serverName *uint16, serverPort uint16, reserved uint32) (handle Handle, err error) = winhttp.WinHttpConnect
//sys winHTTPOpenRequest(connectHandle Handle, verb *uint16, objectName *uint16, version *uint16, referrer *uint16, acceptTypes **uint16, flags uint32) (requestHandle Handle, err error) = winhttp.WinHttpOpenRequest
//sys winHTTPSendRequest(requestHandle Handle, headers *uint16, headersLength uint32, optional *byte, optionalLength uint32, totalLength uint32, context uintptr) (err error) = winhttp.WinHttpSendRequest
//sys winHTTPReceiveResponse(requestHandle Handle, reserved uintptr) (err error) = winhttp.WinHttpReceiveResponse
//sys winHTTPReadData(requestHandle Handle, buffer *byte, bufferSize uint32, bytesRead *uint32) (err error) = winhttp.WinHttpReadData
//sys winHTTPQueryHeaders(requestHandle Handle, infoLevel uint32, name *uint16, buffer unsafe.Pointer, bufferLen *uint32, index *uint32) (err error) = winhttp.WinHttpQueryHeaders
//sys winHTTPSetOption(handle Handle, option uint32, buffer unsafe.Pointer, bufferLen uint32) (err error) = winhttp.WinHttpSetOption
//sys winHTTPQueryOption(handle Handle, option uint32, buffer unsafe.Pointer, bufferLen *uint32) (err error) = winhttp.WinHttpQueryOption
//sys winHTTPSetTimeouts(handle Handle, resolveTimeout int32, connectTimeout int32, sendTimeout int32, receiveTimeout int32) (err error) = winhttp.WinHttpSetTimeouts

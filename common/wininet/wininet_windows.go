package wininet

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modwininet             = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOptionW = modwininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionPerConnectionOption  = 75
	internetOptionSettingsChanged      = 39
	internetOptionRefresh              = 37
	internetOptionProxySettingsChanged = 95
)

const (
	internetPerConnFlags                     = 1
	internetPerConnProxyServer               = 2
	internetPerConnProxyBypass               = 3
	internetPerConnAutoconfigUrl             = 4
	internetPerConnAutodiscoveryFlags        = 5
	internetPerConnAutoconfigSecondaryUrl    = 6
	internetPerConnAutoconfigReloadDelayMins = 7
	internetPerConnAutoconfigLastDetectTime  = 8
	internetPerConnAutoconfigLastDetectUrl   = 9
	internetPerConnFlagsUi                   = 10
	internetOptionProxyUsername              = 43
	internetOptionProxyPassword              = 44
)

const (
	proxyTypeDirect       = 1
	proxyTypeProxy        = 2
	proxyTypeAutoProxyUrl = 4
	proxyTypeAutoDetect   = 8
)

type internetPerConnOptionList struct {
	dwSize        uint32
	pszConnection uintptr
	dwOptionCount uint32
	dwOptionError uint32
	pOptions      uintptr
}

type internetPerConnOption struct {
	dwOption uint32
	value    [8]byte
}

func internetSetOption(option uintptr, lpBuffer uintptr, dwBufferSize uintptr) error {
	r0, _, err := syscall.SyscallN(procInternetSetOptionW.Addr(), 0, option, lpBuffer, dwBufferSize)
	if r0 != 1 {
		return err
	}
	return nil
}

func setOptions(options ...internetPerConnOption) error {
	var optionList internetPerConnOptionList
	optionList.dwSize = uint32(unsafe.Sizeof(optionList))
	optionList.dwOptionCount = uint32(len(options))
	optionList.dwOptionError = 0
	optionList.pOptions = uintptr(unsafe.Pointer(&options[0]))
	err := internetSetOption(internetOptionPerConnectionOption, uintptr(unsafe.Pointer(&optionList)), uintptr(optionList.dwSize))
	if err != nil {
		return os.NewSyscallError("InternetSetOption(Direct)", err)
	}
	err = internetSetOption(internetOptionSettingsChanged, 0, 0)
	if err != nil {
		return os.NewSyscallError("InternetSetOption(SettingsChanged)", err)
	}
	err = internetSetOption(internetOptionProxySettingsChanged, 0, 0)
	if err != nil {
		return os.NewSyscallError("InternetSetOption(ProxySettingsChanged)", err)
	}
	err = internetSetOption(internetOptionRefresh, 0, 0)
	if err != nil {
		return os.NewSyscallError("InternetSetOption(Refresh)", err)
	}
	return nil
}

func ClearSystemProxy() error {
	var flagsOption internetPerConnOption
	flagsOption.dwOption = internetPerConnFlags
	*((*uint32)(unsafe.Pointer(&flagsOption.value))) = proxyTypeDirect | proxyTypeAutoDetect
	return setOptions(flagsOption)
}

func SetSystemProxy(proxy string, bypass string) error {
	var flagsOption internetPerConnOption
	flagsOption.dwOption = internetPerConnFlags
	*((*uint32)(unsafe.Pointer(&flagsOption.value))) = proxyTypeProxy | proxyTypeDirect
	var proxyOption internetPerConnOption
	proxyOption.dwOption = internetPerConnProxyServer
	*((*uintptr)(unsafe.Pointer(&proxyOption.value))) = uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(proxy)))
	var bypassOption internetPerConnOption
	bypassOption.dwOption = internetPerConnProxyBypass
	*((*uintptr)(unsafe.Pointer(&bypassOption.value))) = uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(bypass)))
	return setOptions(flagsOption, proxyOption, bypassOption)
}

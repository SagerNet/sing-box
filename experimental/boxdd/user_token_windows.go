//go:build windows

// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"io"
	"os/user"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/util/winutil"
	"github.com/sagernet/tailscale/util/winutil/winenv"

	winio "github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

const (
	windowsLogonSource         = "singbox"
	kerberosPackageName        = "Kerberos"
	msv1PackageName            = "MICROSOFT_AUTHENTICATION_PACKAGE_V1_0"
	kerbS4ULogon        int32  = 12
	msv1S4ULogonMessage int32  = 12
	s4uCheckLogonHours  uint32 = 0x2
	networkLogon        int32  = 3
	tokenSourceLength          = 8
	seBackupPrivilege          = "SeBackupPrivilege"
	seRestorePrivilege         = "SeRestorePrivilege"
)

type (
	lsaHandle          windows.Handle
	lsaOperationalMode uint32
)

type kerberosS4ULogon struct {
	MessageType int32
	Flags       uint32
	ClientUPN   windows.NTUnicodeString
	ClientRealm windows.NTUnicodeString
}

type msv1S4ULogon struct {
	MessageType       int32
	Flags             uint32
	UserPrincipalName windows.NTUnicodeString
	DomainName        windows.NTUnicodeString
}

type tokenSource struct {
	SourceName       [tokenSourceLength]byte
	SourceIdentifier windows.LUID
}

type quotaLimits struct {
	PagedPoolLimit        uintptr
	NonPagedPoolLimit     uintptr
	MinimumWorkingSetSize uintptr
	MaximumWorkingSetSize uintptr
	PagefileLimit         uintptr
	TimeLimit             int64
}

func acquireWindowsUserSession(requestedUser *user.User) (windows.Token, io.Closer, error) {
	var (
		primaryToken windows.Token
		profile      *winutil.UserProfile
	)
	err := winio.RunWithPrivileges([]string{seTcbPrivilege, seBackupPrivilege, seRestorePrivilege}, func() error {
		impersonationToken, err := logonWindowsUserS4U(requestedUser)
		if err != nil {
			return err
		}
		defer impersonationToken.Close()
		primaryToken, err = duplicatePrimaryToken(impersonationToken)
		if err != nil {
			return err
		}
		tokenUser, err := primaryToken.GetTokenUser()
		if err != nil {
			return E.Cause(err, "query S4U token user")
		}
		if !strings.EqualFold(tokenUser.User.Sid.String(), requestedUser.Uid) {
			return E.New("S4U token identity does not match requested Windows user")
		}
		profile, err = winutil.LoadUserProfile(primaryToken, requestedUser)
		if err != nil {
			return E.Cause(err, "load Windows user profile")
		}
		return nil
	})
	if err != nil {
		if primaryToken != 0 {
			err = E.Errors(err, primaryToken.Close())
		}
		return 0, nil, err
	}
	return primaryToken, common.Closer(func() error {
		profileError := winio.RunWithPrivileges([]string{seBackupPrivilege, seRestorePrivilege}, profile.Close)
		return E.Errors(profileError, primaryToken.Close())
	}), nil
}

func logonWindowsUserS4U(requestedUser *user.User) (token windows.Token, err error) {
	processName, err := windows.NewNTString(windowsLogonSource)
	if err != nil {
		return 0, err
	}
	var (
		handle lsaHandle
		mode   lsaOperationalMode
	)
	status := lsaRegisterLogonProcess(processName, &handle, &mode)
	if status != 0 {
		return 0, E.Cause(status, "register LSA logon process")
	}
	defer func() {
		closeStatus := lsaDeregisterLogonProcess(handle)
		if closeStatus != 0 {
			err = E.Errors(err, E.Cause(closeStatus, "deregister LSA logon process"))
		}
	}()

	username, domainUser, err := classifyWindowsUser(requestedUser.Username)
	if err != nil {
		return 0, err
	}
	var (
		packageName                     string
		authenticationInformation       unsafe.Pointer
		authenticationInformationLength uint32
	)
	if domainUser {
		if !winenv.IsDomainJoined() {
			return 0, E.New("cannot log on as a domain user from a Windows device that is not domain joined")
		}
		packageName = kerberosPackageName
		upn, err := samAccountNameToUPN(username)
		if err != nil {
			return 0, E.Cause(err, "resolve Windows user principal name")
		}
		upn16, err := windows.UTF16FromString(upn)
		if err != nil {
			return 0, err
		}
		logonInfo, logonInfoLen, buffers := winutil.AllocateContiguousBuffer[kerberosS4ULogon](upn16)
		logonInfo.MessageType = kerbS4ULogon
		logonInfo.Flags = s4uCheckLogonHours
		winutil.SetNTString(&logonInfo.ClientUPN, buffers[0])
		authenticationInformation = unsafe.Pointer(logonInfo)
		authenticationInformationLength = logonInfoLen
	} else {
		packageName = msv1PackageName
		username16, err := windows.UTF16FromString(username)
		if err != nil {
			return 0, err
		}
		thisComputer := []uint16{'.', 0}
		logonInfo, logonInfoLen, buffers := winutil.AllocateContiguousBuffer[msv1S4ULogon](username16, thisComputer)
		logonInfo.MessageType = msv1S4ULogonMessage
		logonInfo.Flags = s4uCheckLogonHours
		winutil.SetNTString(&logonInfo.UserPrincipalName, buffers[0])
		winutil.SetNTString(&logonInfo.DomainName, buffers[1])
		authenticationInformation = unsafe.Pointer(logonInfo)
		authenticationInformationLength = logonInfoLen
	}

	packageString, err := windows.NewNTString(packageName)
	if err != nil {
		return 0, err
	}
	var packageID uint32
	status = lsaLookupAuthenticationPackage(handle, packageString, &packageID)
	if status != 0 {
		return 0, E.Cause(status, "lookup LSA authentication package")
	}

	var source tokenSource
	copy(source.SourceName[:], windowsLogonSource)
	err = allocateLocallyUniqueID(&source.SourceIdentifier)
	if err != nil {
		return 0, E.Cause(err, "allocate LSA logon identifier")
	}
	originName, err := windows.NewNTString(windowsLogonSource)
	if err != nil {
		return 0, err
	}
	var (
		profileBuffer       uintptr
		profileBufferLength uint32
		logonID             windows.LUID
		quotas              quotaLimits
		subStatus           windows.NTStatus
	)
	status = lsaLogonUser(
		handle,
		originName,
		networkLogon,
		packageID,
		authenticationInformation,
		authenticationInformationLength,
		nil,
		&source,
		&profileBuffer,
		&profileBufferLength,
		&logonID,
		&token,
		&quotas,
		&subStatus,
	)
	if profileBuffer != 0 {
		defer lsaFreeReturnBuffer(profileBuffer)
	}
	if status != 0 {
		return 0, E.New("S4U logon for ", requestedUser.Username, " failed: ", status, ", substatus: ", subStatus)
	}
	return token, nil
}

func classifyWindowsUser(username string) (sanitizedUsername string, domainUser bool, err error) {
	domain, account, hasDomain := strings.Cut(username, `\`)
	if !hasDomain {
		return username, false, nil
	}
	if domain == "." {
		return account, false, nil
	}
	computerName, err := windows.ComputerName()
	if err != nil {
		return "", false, E.Cause(err, "query Windows computer name")
	}
	if strings.EqualFold(domain, computerName) {
		return account, false, nil
	}
	return username, true, nil
}

func samAccountNameToUPN(samAccountName string) (string, error) {
	_, account, _ := strings.Cut(samAccountName, `\`)
	upn, err := windows.TranslateAccountName(samAccountName, windows.NameSamCompatible, windows.NameUserPrincipal, 50)
	if err == nil {
		return upn, nil
	}
	canonicalName, canonicalError := windows.TranslateAccountName(samAccountName, windows.NameSamCompatible, windows.NameCanonical, 50)
	if canonicalError != nil {
		return "", E.Errors(err, canonicalError)
	}
	domain, _, found := strings.Cut(canonicalName, "/")
	if !found || domain == "" {
		return "", E.New("invalid canonical domain name for ", samAccountName)
	}
	return account + "@" + domain, nil
}

func duplicatePrimaryToken(impersonationToken windows.Token) (windows.Token, error) {
	securityDescriptor, err := windows.GetSecurityInfo(
		windows.Handle(impersonationToken),
		windows.SE_KERNEL_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return 0, E.Cause(err, "query S4U token security")
	}
	securityAttributes := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: securityDescriptor,
	}
	var primaryToken windows.Token
	err = windows.DuplicateTokenEx(
		impersonationToken,
		0,
		&securityAttributes,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&primaryToken,
	)
	if err != nil {
		return 0, E.Cause(err, "duplicate S4U primary token")
	}
	return primaryToken, nil
}

var (
	modAdvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	modSecur32  = windows.NewLazySystemDLL("secur32.dll")

	procAllocateLocallyUniqueID        = modAdvapi32.NewProc("AllocateLocallyUniqueId")
	procLsaDeregisterLogonProcess      = modSecur32.NewProc("LsaDeregisterLogonProcess")
	procLsaFreeReturnBuffer            = modSecur32.NewProc("LsaFreeReturnBuffer")
	procLsaLogonUser                   = modSecur32.NewProc("LsaLogonUser")
	procLsaLookupAuthenticationPackage = modSecur32.NewProc("LsaLookupAuthenticationPackage")
	procLsaRegisterLogonProcess        = modSecur32.NewProc("LsaRegisterLogonProcess")
)

func allocateLocallyUniqueID(luid *windows.LUID) error {
	result, _, callError := syscall.SyscallN(procAllocateLocallyUniqueID.Addr(), uintptr(unsafe.Pointer(luid)))
	if result == 0 {
		if callError == 0 {
			return syscall.EINVAL
		}
		return callError
	}
	return nil
}

func lsaDeregisterLogonProcess(handle lsaHandle) windows.NTStatus {
	result, _, _ := syscall.SyscallN(procLsaDeregisterLogonProcess.Addr(), uintptr(handle))
	return windows.NTStatus(result)
}

func lsaFreeReturnBuffer(buffer uintptr) windows.NTStatus {
	result, _, _ := syscall.SyscallN(procLsaFreeReturnBuffer.Addr(), buffer)
	return windows.NTStatus(result)
}

func lsaLookupAuthenticationPackage(handle lsaHandle, packageName *windows.NTString, packageID *uint32) windows.NTStatus {
	result, _, _ := syscall.SyscallN(
		procLsaLookupAuthenticationPackage.Addr(),
		uintptr(handle),
		uintptr(unsafe.Pointer(packageName)),
		uintptr(unsafe.Pointer(packageID)),
	)
	return windows.NTStatus(result)
}

func lsaRegisterLogonProcess(processName *windows.NTString, handle *lsaHandle, mode *lsaOperationalMode) windows.NTStatus {
	result, _, _ := syscall.SyscallN(
		procLsaRegisterLogonProcess.Addr(),
		uintptr(unsafe.Pointer(processName)),
		uintptr(unsafe.Pointer(handle)),
		uintptr(unsafe.Pointer(mode)),
	)
	return windows.NTStatus(result)
}

func lsaLogonUser(
	handle lsaHandle,
	originName *windows.NTString,
	logonType int32,
	authenticationPackage uint32,
	authenticationInformation unsafe.Pointer,
	authenticationInformationLength uint32,
	localGroups *windows.Tokengroups,
	sourceContext *tokenSource,
	profileBuffer *uintptr,
	profileBufferLength *uint32,
	logonID *windows.LUID,
	token *windows.Token,
	quotas *quotaLimits,
	subStatus *windows.NTStatus,
) windows.NTStatus {
	result, _, _ := syscall.SyscallN(
		procLsaLogonUser.Addr(),
		uintptr(handle),
		uintptr(unsafe.Pointer(originName)),
		uintptr(logonType),
		uintptr(authenticationPackage),
		uintptr(authenticationInformation),
		uintptr(authenticationInformationLength),
		uintptr(unsafe.Pointer(localGroups)),
		uintptr(unsafe.Pointer(sourceContext)),
		uintptr(unsafe.Pointer(profileBuffer)),
		uintptr(unsafe.Pointer(profileBufferLength)),
		uintptr(unsafe.Pointer(logonID)),
		uintptr(unsafe.Pointer(token)),
		uintptr(unsafe.Pointer(quotas)),
		uintptr(unsafe.Pointer(subStatus)),
	)
	return windows.NTStatus(result)
}

//go:build windows

package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/sagernet/sing-box/common/badversion"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/libbox"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	updateInstallerDesktop = `winsta0\default`
	updateProductName      = "sing-box"
	seTcbPrivilege         = "SeTcbPrivilege"
	seAssignPrimaryToken   = "SeAssignPrimaryTokenPrivilege"
	seIncreaseQuota        = "SeIncreaseQuotaPrivilege"
)

func (d *Daemon) installUpdate(identity peerIdentity, installerPath string) (*InstallUpdateResponse, error) {
	if installerPath == "" {
		return nil, status.Error(codes.InvalidArgument, "missing update installer path")
	}
	platform := d.platform.(*windowsPlatformInterface)
	platform.updateAccess.Lock()
	if platform.updateInProgress {
		platform.updateAccess.Unlock()
		return nil, status.Error(codes.Aborted, "update installation is already in progress")
	}
	platform.updateInProgress = true
	platform.updateAccess.Unlock()
	started := false
	defer func() {
		if started {
			return
		}
		platform.updateAccess.Lock()
		platform.updateInProgress = false
		platform.updateAccess.Unlock()
	}()

	installer, err := openLockedExecutable(installerPath)
	if err != nil {
		return nil, E.Cause(err, "open update installer")
	}
	defer windows.CloseHandle(installer)
	installerFinalPath, err := finalWindowsPath(installer)
	if err != nil {
		return nil, E.Cause(err, "resolve update installer")
	}
	installerIdentity, err := windowsExecutableIdentity(installerFinalPath)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, E.Cause(err, "read update installer identity").Error())
	}
	if installerIdentity.productName != updateProductName {
		return nil, status.Error(codes.InvalidArgument, "update executable is not a sing-box installer")
	}
	err = validateNSISExecutable(installerFinalPath)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, E.Cause(err, "update executable is not a sing-box installer").Error())
	}
	installerSigner, err := authenticodeSigner(installerFinalPath, installer)
	if err != nil {
		return nil, E.Cause(err, "authenticate update installer")
	}
	if !bytes.Equal(installerSigner, platform.daemonSigner) {
		return &InstallUpdateResponse{Result: InstallUpdateResult_INSTALL_UPDATE_RESULT_SIGNER_MISMATCH}, nil
	}
	if !badversion.IsValid(installerIdentity.version) {
		return nil, status.Error(codes.InvalidArgument, "update installer has an invalid version")
	}
	if !libbox.CompareSemver(installerIdentity.version, C.Version) {
		return &InstallUpdateResponse{Result: InstallUpdateResult_INSTALL_UPDATE_RESULT_NOT_NEWER}, nil
	}
	installerProcess, err := launchUpdateInstaller(installerFinalPath, identity.SessionID)
	if err != nil {
		return nil, E.Cause(err, "launch update installer")
	}
	started = true
	go platform.waitUpdateInstaller(installerProcess)
	return &InstallUpdateResponse{Result: InstallUpdateResult_INSTALL_UPDATE_RESULT_STARTED}, nil
}

func (p *windowsPlatformInterface) waitUpdateInstaller(process windows.Handle) {
	defer windows.CloseHandle(process)
	_, _ = windows.WaitForSingleObject(process, windows.INFINITE)
	p.updateAccess.Lock()
	p.updateInProgress = false
	p.updateAccess.Unlock()
}

type windowsExecutableVersionIdentity struct {
	productName string
	version     string
}

type windowsVersionTranslation struct {
	language uint16
	codePage uint16
}

func windowsExecutableIdentity(path string) (windowsExecutableVersionIdentity, error) {
	var zero windows.Handle
	informationSize, err := windows.GetFileVersionInfoSize(path, &zero)
	if err != nil {
		return windowsExecutableVersionIdentity{}, err
	}
	information := make([]byte, informationSize)
	err = windows.GetFileVersionInfo(path, 0, informationSize, unsafe.Pointer(&information[0]))
	if err != nil {
		return windowsExecutableVersionIdentity{}, err
	}
	var translationsPointer *windowsVersionTranslation
	var translationsSize uint32
	err = windows.VerQueryValue(
		unsafe.Pointer(&information[0]),
		`\VarFileInfo\Translation`,
		unsafe.Pointer(&translationsPointer),
		&translationsSize,
	)
	if err != nil {
		return windowsExecutableVersionIdentity{}, E.Cause(err, "query version translations")
	}
	if translationsPointer == nil || translationsSize == 0 || translationsSize%uint32(unsafe.Sizeof(windowsVersionTranslation{})) != 0 {
		return windowsExecutableVersionIdentity{}, E.New("invalid version translations")
	}
	translations := unsafe.Slice(
		translationsPointer,
		int(translationsSize/uint32(unsafe.Sizeof(windowsVersionTranslation{}))),
	)
	for _, translation := range translations {
		productName, productErr := windowsVersionString(information, translation, "ProductName")
		if productErr != nil {
			continue
		}
		version, versionErr := windowsVersionString(information, translation, "FileVersion")
		if versionErr != nil {
			continue
		}
		return windowsExecutableVersionIdentity{
			productName: strings.TrimSpace(productName),
			version:     strings.TrimSpace(version),
		}, nil
	}
	return windowsExecutableVersionIdentity{}, E.New("missing product name or file version")
}

func windowsVersionString(information []byte, translation windowsVersionTranslation, name string) (string, error) {
	query := fmt.Sprintf(`\StringFileInfo\%04X%04X\%s`, translation.language, translation.codePage, name)
	var valuePointer *uint16
	var valueLength uint32
	err := windows.VerQueryValue(
		unsafe.Pointer(&information[0]),
		query,
		unsafe.Pointer(&valuePointer),
		&valueLength,
	)
	if err != nil {
		return "", err
	}
	if valuePointer == nil || valueLength == 0 {
		return "", E.New("empty version string")
	}
	return windows.UTF16ToString(unsafe.Slice(valuePointer, int(valueLength))), nil
}

func launchUpdateInstaller(installerPath string, sessionID uint32) (windows.Handle, error) {
	var processToken windows.Token
	err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY,
		&processToken,
	)
	if err != nil {
		return 0, E.Cause(err, "open daemon process token")
	}
	defer processToken.Close()
	var installerToken windows.Token
	err = windows.DuplicateTokenEx(
		processToken,
		windows.TOKEN_ALL_ACCESS,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&installerToken,
	)
	if err != nil {
		return 0, E.Cause(err, "duplicate daemon process token")
	}
	defer installerToken.Close()
	var installerProcess windows.Handle
	err = winio.RunWithPrivileges(
		[]string{seTcbPrivilege, seAssignPrimaryToken, seIncreaseQuota},
		func() error {
			err = windows.SetTokenInformation(
				installerToken,
				windows.TokenSessionId,
				(*byte)(unsafe.Pointer(&sessionID)),
				uint32(unsafe.Sizeof(sessionID)),
			)
			if err != nil {
				return E.Cause(err, "set update installer session")
			}
			installerProcess, err = createUpdateInstallerProcess(installerToken, installerPath)
			return err
		},
	)
	return installerProcess, err
}

func createUpdateInstallerProcess(token windows.Token, installerPath string) (windows.Handle, error) {
	applicationName, err := windows.UTF16PtrFromString(installerPath)
	if err != nil {
		return 0, err
	}
	commandLine, err := windows.UTF16FromString(windows.ComposeCommandLine([]string{
		installerPath,
		"--updated",
		"/S",
		"--force-run",
	}))
	if err != nil {
		return 0, err
	}
	desktop, err := windows.UTF16PtrFromString(updateInstallerDesktop)
	if err != nil {
		return 0, err
	}
	workingDirectory, err := windows.UTF16PtrFromString(filepath.Dir(installerPath))
	if err != nil {
		return 0, err
	}
	startupInformation := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktop,
	}
	var processInformation windows.ProcessInformation
	err = windows.CreateProcessAsUser(
		token,
		applicationName,
		&commandLine[0],
		nil,
		nil,
		false,
		windows.CREATE_NEW_PROCESS_GROUP|windows.CREATE_UNICODE_ENVIRONMENT,
		nil,
		workingDirectory,
		&startupInformation,
		&processInformation,
	)
	if err != nil {
		return 0, err
	}
	windows.CloseHandle(processInformation.Thread)
	return processInformation.Process, nil
}

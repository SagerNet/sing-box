//go:build windows

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	trustedInstallerUserID = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	fileDeleteChildAccess  = 0x00000040
)

func secureWindowsInstallation(executablePath string, allowUnsafeInstallation bool) (string, error) {
	daemonExecutable, err := openLockedExecutable(executablePath)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(daemonExecutable)
	daemonPath, err := finalWindowsPath(daemonExecutable)
	if err != nil {
		return "", err
	}
	installationDirectory, applicationPath, err := installedApplicationPath(daemonPath)
	if err != nil {
		return "", err
	}
	applicationExecutable, err := openLockedExecutable(applicationPath)
	if err != nil {
		return "", E.Cause(err, "open installed application")
	}
	defer windows.CloseHandle(applicationExecutable)
	daemonSigner, err := authenticodeSigner(daemonPath, daemonExecutable)
	if err != nil {
		return "", E.Cause(err, "authenticate installed daemon")
	}
	applicationFinalPath, err := finalWindowsPath(applicationExecutable)
	if err != nil {
		return "", err
	}
	applicationSigner, err := authenticodeSigner(applicationFinalPath, applicationExecutable)
	if err != nil {
		return "", E.Cause(err, "authenticate installed application")
	}
	if !bytes.Equal(daemonSigner, applicationSigner) {
		return "", E.New("installed application and daemon have different signing certificates")
	}
	if allowUnsafeInstallation {
		return daemonPath, nil
	}
	volumeRoot, err := validateFixedNTFSVolume(installationDirectory)
	if err != nil {
		return "", err
	}
	err = validateInstallationAncestors(filepath.Dir(installationDirectory), volumeRoot, false)
	if err != nil {
		return "", err
	}
	err = validateTreeHasNoReparsePoints(installationDirectory)
	if err != nil {
		return "", err
	}
	err = applyProtectedTree(
		installationDirectory,
		"O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;AU)",
		"O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;GRGX;;;AU)",
	)
	if err != nil {
		return "", err
	}
	return daemonPath, nil
}

func installedApplicationPath(daemonPath string) (string, string, error) {
	daemonDirectory := filepath.Dir(daemonPath)
	resourcesDirectory := filepath.Dir(daemonDirectory)
	installationDirectory := filepath.Dir(resourcesDirectory)
	if !strings.EqualFold(filepath.Base(daemonPath), daemonExecutableName) ||
		!strings.EqualFold(filepath.Base(daemonDirectory), "daemon") ||
		!strings.EqualFold(filepath.Base(resourcesDirectory), "resources") {
		return "", "", E.New("daemon executable is outside the installed sing-box layout")
	}
	return installationDirectory, filepath.Join(installationDirectory, applicationExecutableName), nil
}

func windowsWorkingDirectorySecurityDescriptors(serviceUserID *windows.SID) (string, string, error) {
	serviceUserIDString := serviceUserID.String()
	if serviceUserIDString == "" {
		return "", "", E.New("daemon service has an invalid SID")
	}
	directoryDescriptor := fmt.Sprintf(
		"O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)",
		serviceUserIDString,
	)
	fileDescriptor := fmt.Sprintf(
		"O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;%s)",
		serviceUserIDString,
	)
	return directoryDescriptor, fileDescriptor, nil
}

func secureWindowsWorkingDirectory(path string, serviceUserID *windows.SID) error {
	directoryDescriptor, fileDescriptor, err := windowsWorkingDirectorySecurityDescriptors(serviceUserID)
	if err != nil {
		return err
	}
	err = validateTreeHasNoReparsePoints(path)
	if err != nil {
		return err
	}
	return applyProtectedTree(path, directoryDescriptor, fileDescriptor)
}

func secureWindowsWorkingDirectoryRoot(path string, serviceUserID *windows.SID) error {
	directorySecurityDescriptor, _, err := windowsWorkingDirectorySecurityDescriptors(serviceUserID)
	if err != nil {
		return err
	}
	directoryDescriptor, err := windows.SecurityDescriptorFromString(directorySecurityDescriptor)
	if err != nil {
		return err
	}
	return winio.RunWithPrivilege(winio.SeRestorePrivilege, func() error {
		return applyProtectedFileSecurity(path, directoryDescriptor)
	})
}

func windowsServiceSID() (*windows.SID, error) {
	serviceNameUTF16 := utf16.Encode([]rune(strings.ToUpper(serviceName)))
	serviceNameContent := make([]byte, len(serviceNameUTF16)*2)
	for index, codeUnit := range serviceNameUTF16 {
		binary.LittleEndian.PutUint16(serviceNameContent[index*2:], codeUnit)
	}
	serviceNameHash := sha1.Sum(serviceNameContent)
	return windows.StringToSid(fmt.Sprintf(
		"S-1-5-80-%d-%d-%d-%d-%d",
		binary.LittleEndian.Uint32(serviceNameHash[0:4]),
		binary.LittleEndian.Uint32(serviceNameHash[4:8]),
		binary.LittleEndian.Uint32(serviceNameHash[8:12]),
		binary.LittleEndian.Uint32(serviceNameHash[12:16]),
		binary.LittleEndian.Uint32(serviceNameHash[16:20]),
	))
}

func validateProtectedWindowsWorkingDirectory(path string, serviceUserID *windows.SID) error {
	return validateWindowsWorkingDirectory(path, serviceUserID, false)
}

func validateRepairableWindowsWorkingDirectory(path string, serviceUserID *windows.SID) error {
	return validateWindowsWorkingDirectory(path, serviceUserID, true)
}

func validateWindowsWorkingDirectory(path string, serviceUserID *windows.SID, allowAdditionalAccessControlEntries bool) error {
	attributes, err := windowsFileAttributes(path)
	if err != nil {
		return err
	}
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return E.New("daemon working directory path is not a directory")
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return E.New("daemon working directory is a reparse point")
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	systemUserID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	if !windows.EqualSid(owner, systemUserID) {
		return E.New("daemon working directory is not owned by SYSTEM")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return E.New("daemon working directory access control is inherited")
	}
	discretionaryAccessControlList, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if discretionaryAccessControlList == nil ||
		(!allowAdditionalAccessControlEntries && discretionaryAccessControlList.AceCount != 3) ||
		(allowAdditionalAccessControlEntries && discretionaryAccessControlList.AceCount < 3) {
		return E.New("daemon working directory has unexpected access control entries")
	}
	administratorsUserID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	expectedUsers := map[string]bool{
		systemUserID.String():         false,
		administratorsUserID.String(): false,
		serviceUserID.String():        false,
	}
	for index := uint32(0); index < uint32(discretionaryAccessControlList.AceCount); index++ {
		var accessControlEntry *windows.ACCESS_ALLOWED_ACE
		err = windows.GetAce(discretionaryAccessControlList, index, &accessControlEntry)
		if err != nil {
			return err
		}
		if accessControlEntry.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return E.New("daemon working directory has an unsafe access control entry")
		}
		userID := (*windows.SID)(unsafe.Pointer(&accessControlEntry.SidStart)).String()
		seen, exists := expectedUsers[userID]
		if !exists {
			if allowAdditionalAccessControlEntries {
				continue
			}
			return E.New("daemon working directory grants access to an unexpected principal")
		}
		if accessControlEntry.Header.AceFlags != windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE ||
			uint32(accessControlEntry.Mask) != 0x001F01FF {
			return E.New("daemon working directory has an unsafe access control entry")
		}
		if seen {
			return E.New("daemon working directory has a duplicate access control entry")
		}
		expectedUsers[userID] = true
	}
	for _, seen := range expectedUsers {
		if !seen {
			return E.New("daemon working directory is missing a required access control entry")
		}
	}
	return nil
}

func applyProtectedServiceSecurity(service *mgr.Service) error {
	descriptor, err := windows.SecurityDescriptorFromString(
		"D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x2008d;;;AU)",
	)
	if err != nil {
		return err
	}
	discretionaryAccessControlList, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(
		service.Handle,
		windows.SE_SERVICE,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		discretionaryAccessControlList,
		nil,
	)
}

func allowAuthenticatedUsersToQueryCurrentProcess() error {
	descriptor, err := windows.SecurityDescriptorFromString(
		"D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;0x101000;;;AU)",
	)
	if err != nil {
		return err
	}
	discretionaryAccessControlList, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(
		windows.CurrentProcess(),
		windows.SE_KERNEL_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		discretionaryAccessControlList,
		nil,
	)
}

func validateFixedNTFSVolume(path string) (string, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	volumePathBuffer := make([]uint16, windows.MAX_LONG_PATH)
	err = windows.GetVolumePathName(pathPointer, &volumePathBuffer[0], uint32(len(volumePathBuffer)))
	if err != nil {
		return "", E.Cause(err, "resolve installation volume")
	}
	volumePath := windows.UTF16ToString(volumePathBuffer)
	volumePathPointer, err := windows.UTF16PtrFromString(volumePath)
	if err != nil {
		return "", err
	}
	if windows.GetDriveType(volumePathPointer) != windows.DRIVE_FIXED {
		return "", E.New("sing-box must be installed on a fixed local drive")
	}
	fileSystemNameBuffer := make([]uint16, 32)
	err = windows.GetVolumeInformation(
		volumePathPointer,
		nil,
		0,
		nil,
		nil,
		nil,
		&fileSystemNameBuffer[0],
		uint32(len(fileSystemNameBuffer)),
	)
	if err != nil {
		return "", E.Cause(err, "query installation file system")
	}
	if !strings.EqualFold(windows.UTF16ToString(fileSystemNameBuffer), "NTFS") {
		return "", E.New("sing-box must be installed on NTFS")
	}
	return filepath.Clean(volumePath), nil
}

func resolveWindowsServiceWorkingDirectory(path string, allowUnsafePermissions bool) (string, error) {
	if path == "" {
		return "", E.New("missing daemon working directory")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", E.Cause(err, "resolve daemon working directory")
	}
	cleanPath := filepath.Clean(absolutePath)
	parentPath := filepath.Dir(cleanPath)
	parentAttributes, err := windowsFileAttributes(parentPath)
	if err != nil {
		return "", E.Cause(err, "query daemon working directory parent")
	}
	if parentAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return "", E.New("daemon working directory parent is not a directory")
	}
	if parentAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", E.New("daemon working directory parent is a reparse point")
	}
	volumeRoot, err := validateFixedNTFSVolume(parentPath)
	if err != nil {
		return "", E.Cause(err, "validate daemon working directory volume")
	}
	if strings.EqualFold(cleanPath, filepath.Clean(volumeRoot)) {
		return "", E.New("daemon working directory must not be a volume root")
	}
	err = validateInstallationAncestors(parentPath, volumeRoot, allowUnsafePermissions)
	if err != nil {
		return "", E.Cause(err, "validate daemon working directory ancestors")
	}
	return cleanPath, nil
}

func validateInstallationAncestors(path string, volumeRoot string, allowUnsafePermissions bool) error {
	currentPath := filepath.Clean(path)
	cleanVolumeRoot := filepath.Clean(volumeRoot)
	for {
		err := validateInstallationAncestor(currentPath, allowUnsafePermissions)
		if err != nil {
			return err
		}
		if strings.EqualFold(currentPath, cleanVolumeRoot) {
			return nil
		}
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			return E.New("installation path is outside its resolved volume")
		}
		currentPath = parentPath
	}
}

func validateInstallationAncestor(path string, allowUnsafePermissions bool) error {
	attributes, err := windowsFileAttributes(path)
	if err != nil {
		return err
	}
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return E.New("installation ancestor is not a directory: ", path)
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return E.New("installation ancestor is a reparse point: ", path)
	}
	if allowUnsafePermissions {
		return nil
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return E.Cause(err, "query installation ancestor security")
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	if !trustedAdministrativeUser(owner) {
		return E.New("installation ancestor is owned by an unprivileged principal: ", path)
	}
	discretionaryAccessControlList, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if discretionaryAccessControlList == nil {
		return E.New("installation ancestor has an empty access control list: ", path)
	}
	for index := uint32(0); index < uint32(discretionaryAccessControlList.AceCount); index++ {
		var accessControlEntry *windows.ACCESS_ALLOWED_ACE
		err = windows.GetAce(discretionaryAccessControlList, index, &accessControlEntry)
		if err != nil {
			return err
		}
		if accessControlEntry.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if accessControlEntry.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		mask := uint32(accessControlEntry.Mask)
		dangerousAccess := uint32(windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL | fileDeleteChildAccess)
		if mask&dangerousAccess == 0 {
			continue
		}
		principal := (*windows.SID)(unsafe.Pointer(&accessControlEntry.SidStart))
		if !trustedAdministrativeUser(principal) {
			return E.New("installation ancestor is replaceable by an unprivileged principal: ", path)
		}
	}
	return nil
}

func trustedAdministrativeUser(userID *windows.SID) bool {
	if userID == nil {
		return false
	}
	return userID.IsWellKnown(windows.WinLocalSystemSid) ||
		userID.IsWellKnown(windows.WinBuiltinAdministratorsSid) ||
		userID.String() == trustedInstallerUserID
}

func validateTreeHasNoReparsePoints(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		attributes, err := windowsFileAttributes(path)
		if err != nil {
			return err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return E.New("protected tree contains a reparse point: ", path)
		}
		return nil
	})
}

func applyProtectedTree(root string, directorySecurityDescriptor string, fileSecurityDescriptor string) error {
	return winio.RunWithPrivilege(winio.SeRestorePrivilege, func() error {
		directoryDescriptor, err := windows.SecurityDescriptorFromString(directorySecurityDescriptor)
		if err != nil {
			return err
		}
		fileDescriptor, err := windows.SecurityDescriptorFromString(fileSecurityDescriptor)
		if err != nil {
			return err
		}
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
			if walkError != nil {
				return walkError
			}
			descriptor := fileDescriptor
			if entry.IsDir() {
				descriptor = directoryDescriptor
			}
			err = applyProtectedFileSecurity(path, descriptor)
			if err != nil {
				return E.Cause(err, "secure ", path)
			}
			return nil
		})
	})
}

func applyProtectedFileSecurity(path string, descriptor *windows.SECURITY_DESCRIPTOR) error {
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	group, _, err := descriptor.Group()
	if err != nil {
		return err
	}
	discretionaryAccessControlList, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|
			windows.GROUP_SECURITY_INFORMATION|
			windows.DACL_SECURITY_INFORMATION|
			windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner,
		group,
		discretionaryAccessControlList,
		nil,
	)
}

func windowsFileAttributes(path string) (uint32, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.GetFileAttributes(pathPointer)
}

func ensureWindowsWorkingDirectory(path string) error {
	serviceUserID, err := windowsServiceSID()
	if err != nil {
		return E.Cause(err, "create daemon service SID")
	}
	created := false
	_, err = os.Lstat(path)
	if os.IsNotExist(err) {
		serviceUserIDString := serviceUserID.String()
		if serviceUserIDString == "" {
			return E.New("daemon service has an invalid SID")
		}
		descriptor, descriptorError := windows.SecurityDescriptorFromString(
			fmt.Sprintf("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;%s)", serviceUserIDString),
		)
		if descriptorError != nil {
			return descriptorError
		}
		pathPointer, pathError := windows.UTF16PtrFromString(path)
		if pathError != nil {
			return pathError
		}
		securityAttributes := &windows.SecurityAttributes{
			Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
			SecurityDescriptor: descriptor,
		}
		err = windows.CreateDirectory(pathPointer, securityAttributes)
		if err != nil {
			return E.Cause(err, "create protected daemon working directory")
		}
		created = true
	} else if err != nil {
		return err
	}
	if !created {
		err = validateRepairableWindowsWorkingDirectory(path, serviceUserID)
		if err != nil {
			return err
		}
		err = secureWindowsWorkingDirectoryRoot(path, serviceUserID)
		if err != nil {
			return err
		}
	}
	err = secureWindowsWorkingDirectory(path, serviceUserID)
	if err != nil {
		return err
	}
	return validateProtectedWindowsWorkingDirectory(path, serviceUserID)
}

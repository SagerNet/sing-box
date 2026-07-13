//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	winioProcess "github.com/tailscale/go-winio/pkg/process"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	daemonExecutableName      = "sing-box-daemon.exe"
	applicationExecutableName = "sing-box.exe"
	workerPipePrefix          = `\\.\pipe\sing-box-worker.`
)

type windowsTransportCredentials struct {
	daemon                  *Daemon
	daemonSigner            []byte
	daemonExecutable        windows.Handle
	expectedWorkerPath      string
	expectedApplicationPath string
}

type windowsAuthenticatedConnection struct {
	net.Conn
	daemon             *Daemon
	identity           peerIdentity
	process            windows.Handle
	processImage       windows.Handle
	parentProcess      windows.Handle
	parentProcessImage windows.Handle
	close              sync.Once
	closeError         error
}

type fileDescriptorConnection interface {
	Fd() uintptr
}

func platformServerOptions(daemon *Daemon) ([]grpc.ServerOption, error) {
	if listenAddress != "" {
		return nil, nil
	}
	transportCredentials := &windowsTransportCredentials{daemon: daemon}
	err := transportCredentials.initializeServerIdentity()
	if err != nil {
		return nil, err
	}
	return []grpc.ServerOption{grpc.Creds(transportCredentials)}, nil
}

func platformFallbackPeerIdentity(ctx context.Context) (peerIdentity, error) {
	return peerIdentity{}, E.New("missing Windows peer authentication")
}

func (c *windowsTransportCredentials) ClientHandshake(ctx context.Context, authority string, rawConnection net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, E.New("Windows local process credentials do not support client handshakes")
}

func (c *windowsTransportCredentials) ServerHandshake(rawConnection net.Conn) (net.Conn, credentials.AuthInfo, error) {
	connection, authenticationInformation, err := c.serverHandshake(rawConnection)
	if err != nil {
		serviceLogError(E.Cause(err, "reject Windows daemon connection"))
	}
	return connection, authenticationInformation, err
}

func (c *windowsTransportCredentials) serverHandshake(rawConnection net.Conn) (net.Conn, credentials.AuthInfo, error) {
	descriptorConnection, loaded := rawConnection.(fileDescriptorConnection)
	if !loaded {
		return nil, nil, E.New("daemon endpoint is not a Windows named pipe")
	}
	var processID uint32
	err := windows.GetNamedPipeClientProcessId(windows.Handle(descriptorConnection.Fd()), &processID)
	if err != nil {
		return nil, nil, E.Cause(err, "identify named pipe client")
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, processID)
	if err != nil {
		return nil, nil, E.Cause(err, "open named pipe client process")
	}
	keepProcess := false
	defer func() {
		if !keepProcess {
			windows.CloseHandle(process)
		}
	}()
	identity, err := processIdentity(process, processID)
	if err != nil {
		return nil, nil, err
	}
	workerImagePath, err := winioProcess.QueryFullProcessImageName(process, winioProcess.ImageNameFormatWin32Path)
	if err != nil {
		return nil, nil, E.Cause(err, "query named pipe client executable")
	}
	processImage, err := openLockedExecutable(workerImagePath)
	if err != nil {
		return nil, nil, E.Cause(err, "open named pipe client executable")
	}
	keepProcessImage := false
	defer func() {
		if !keepProcessImage {
			windows.CloseHandle(processImage)
		}
	}()
	processImageFinalPath, err := finalWindowsPath(processImage)
	if err != nil {
		return nil, nil, E.Cause(err, "resolve named pipe client executable")
	}
	if !strings.EqualFold(processImageFinalPath, c.expectedWorkerPath) {
		return nil, nil, E.New("named pipe client is not the installed sing-box worker")
	}
	sameExecutable, err := sameWindowsFile(processImage, c.daemonExecutable)
	if err != nil {
		return nil, nil, err
	}
	if !sameExecutable {
		return nil, nil, E.New("named pipe client worker executable was replaced")
	}
	workerSigner, err := authenticodeSigner(processImageFinalPath, processImage)
	if err != nil {
		return nil, nil, E.Cause(err, "authenticate sing-box worker")
	}
	if !bytes.Equal(workerSigner, c.daemonSigner) {
		return nil, nil, E.New("sing-box worker and daemon have different signing certificates")
	}
	parentProcessID, err := processParentID(process)
	if err != nil {
		return nil, nil, err
	}
	err = validateWorkerProcessRole(process, parentProcessID)
	if err != nil {
		return nil, nil, err
	}
	parentProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, parentProcessID)
	if err != nil {
		return nil, nil, E.Cause(err, "open sing-box worker parent process")
	}
	keepParentProcess := false
	defer func() {
		if !keepParentProcess {
			windows.CloseHandle(parentProcess)
		}
	}()
	parentIdentity, err := processIdentity(parentProcess, parentProcessID)
	if err != nil {
		return nil, nil, err
	}
	if parentIdentity.UserID != identity.UserID || parentIdentity.SessionID != identity.SessionID {
		return nil, nil, E.New("sing-box worker and application have different process identities")
	}
	parentCreationTime, err := processCreationTime(parentProcess)
	if err != nil {
		return nil, nil, err
	}
	workerCreationTime, err := processCreationTime(process)
	if err != nil {
		return nil, nil, err
	}
	if parentCreationTime >= workerCreationTime {
		return nil, nil, E.New("sing-box worker parent was created after the worker")
	}
	parentImagePath, err := winioProcess.QueryFullProcessImageName(parentProcess, winioProcess.ImageNameFormatWin32Path)
	if err != nil {
		return nil, nil, E.Cause(err, "query sing-box worker parent executable")
	}
	parentProcessImage, err := openLockedExecutable(parentImagePath)
	if err != nil {
		return nil, nil, E.Cause(err, "open sing-box worker parent executable")
	}
	keepParentProcessImage := false
	defer func() {
		if !keepParentProcessImage {
			windows.CloseHandle(parentProcessImage)
		}
	}()
	expectedApplication, err := openLockedExecutable(c.expectedApplicationPath)
	if err != nil {
		return nil, nil, E.Cause(err, "open installed application executable")
	}
	defer windows.CloseHandle(expectedApplication)
	parentImageFinalPath, err := finalWindowsPath(parentProcessImage)
	if err != nil {
		return nil, nil, E.Cause(err, "resolve sing-box worker parent executable")
	}
	expectedApplicationFinalPath, err := finalWindowsPath(expectedApplication)
	if err != nil {
		return nil, nil, E.Cause(err, "resolve installed application executable")
	}
	if !strings.EqualFold(parentImageFinalPath, expectedApplicationFinalPath) {
		return nil, nil, E.New("sing-box worker parent is not the installed application")
	}
	sameApplication, err := sameWindowsFile(parentProcessImage, expectedApplication)
	if err != nil {
		return nil, nil, err
	}
	if !sameApplication {
		return nil, nil, E.New("sing-box worker parent executable was replaced")
	}
	err = validateApplicationProcessRole(parentProcess, expectedApplication)
	if err != nil {
		return nil, nil, err
	}
	applicationSigner, err := authenticodeSigner(parentImageFinalPath, parentProcessImage)
	if err != nil {
		return nil, nil, E.Cause(err, "authenticate sing-box application")
	}
	if !bytes.Equal(applicationSigner, c.daemonSigner) {
		return nil, nil, E.New("sing-box application and daemon have different signing certificates")
	}
	workerWaitResult, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		return nil, nil, err
	}
	parentWaitResult, err := windows.WaitForSingleObject(parentProcess, 0)
	if err != nil {
		return nil, nil, err
	}
	if workerWaitResult != uint32(windows.WAIT_TIMEOUT) || parentWaitResult != uint32(windows.WAIT_TIMEOUT) {
		return nil, nil, E.New("sing-box worker or application exited during authentication")
	}
	parentIdentity.ProcessID = parentProcessID
	connection := &windowsAuthenticatedConnection{
		Conn:               rawConnection,
		daemon:             c.daemon,
		identity:           parentIdentity,
		process:            process,
		processImage:       processImage,
		parentProcess:      parentProcess,
		parentProcessImage: parentProcessImage,
	}
	keepProcess = true
	keepProcessImage = true
	keepParentProcess = true
	keepParentProcessImage = true
	c.daemon.registerPeerConnection(connection)
	authenticationInformation := &peerAuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		identity:       parentIdentity,
	}
	return connection, authenticationInformation, nil
}

func (c *windowsTransportCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "windows-local-process",
		SecurityVersion:  "1",
	}
}

func (c *windowsTransportCredentials) Clone() credentials.TransportCredentials {
	return &windowsTransportCredentials{
		daemon:                  c.daemon,
		daemonSigner:            c.daemonSigner,
		daemonExecutable:        c.daemonExecutable,
		expectedWorkerPath:      c.expectedWorkerPath,
		expectedApplicationPath: c.expectedApplicationPath,
	}
}

func (c *windowsTransportCredentials) OverrideServerName(serverNameOverride string) error {
	return nil
}

func (c *windowsTransportCredentials) initializeServerIdentity() error {
	executablePath, err := os.Executable()
	if err != nil {
		return E.Cause(err, "locate daemon executable")
	}
	executable, err := openLockedExecutable(executablePath)
	if err != nil {
		return E.Cause(err, "open daemon executable")
	}
	keepExecutable := false
	defer func() {
		if !keepExecutable {
			windows.CloseHandle(executable)
		}
	}()
	finalPath, err := finalWindowsPath(executable)
	if err != nil {
		return E.Cause(err, "resolve daemon executable")
	}
	_, applicationPath, err := installedApplicationPath(finalPath)
	if err != nil {
		return err
	}
	c.daemonExecutable = executable
	c.expectedWorkerPath = finalPath
	c.expectedApplicationPath = applicationPath
	c.daemonSigner, err = authenticodeSigner(finalPath, executable)
	if err != nil {
		return E.Cause(err, "authenticate daemon executable")
	}
	keepExecutable = true
	return nil
}

func processIdentity(process windows.Handle, processID uint32) (peerIdentity, error) {
	var token windows.Token
	err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token)
	if err != nil {
		return peerIdentity{}, E.Cause(err, "open named pipe client token")
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return peerIdentity{}, E.Cause(err, "query named pipe client user")
	}
	userID := user.User.Sid.String()
	if userID == "" {
		return peerIdentity{}, E.New("named pipe client has an invalid user SID")
	}
	var sessionID uint32
	err = windows.ProcessIdToSessionId(processID, &sessionID)
	if err != nil {
		return peerIdentity{}, E.Cause(err, "query named pipe client session")
	}
	if sessionID == 0 {
		return peerIdentity{}, E.New("named pipe client is not in an interactive session")
	}
	return peerIdentity{UserID: userID, ProcessID: processID, SessionID: sessionID}, nil
}

func validateApplicationProcessRole(process windows.Handle, expectedApplication windows.Handle) error {
	arguments, err := processCommandLine(process)
	if err != nil {
		return E.Cause(err, "query named pipe client command line")
	}
	for _, argument := range arguments[1:] {
		normalizedArgument := strings.ToLower(argument)
		if normalizedArgument == "--type" || strings.HasPrefix(normalizedArgument, "--type=") {
			return E.New("named pipe client is an Electron child process")
		}
	}
	applicationParent, err := processParentIsApplication(process, expectedApplication)
	if err != nil {
		return err
	}
	if applicationParent {
		return E.New("named pipe client is a child of the sing-box application")
	}
	return nil
}

func validateWorkerProcessRole(process windows.Handle, parentProcessID uint32) error {
	arguments, err := processCommandLine(process)
	if err != nil {
		return E.Cause(err, "query sing-box worker command line")
	}
	if len(arguments) != 8 ||
		arguments[1] != "worker" ||
		arguments[2] != "--socket" ||
		arguments[4] != "--parent-pid" ||
		arguments[6] != "--daemon-relay-socket" {
		return E.New("named pipe client is not a sing-box worker process")
	}
	if !strings.HasPrefix(strings.ToLower(arguments[3]), strings.ToLower(workerPipePrefix)) ||
		!strings.HasPrefix(strings.ToLower(arguments[7]), strings.ToLower(workerPipePrefix)) ||
		strings.EqualFold(arguments[3], arguments[7]) {
		return E.New("sing-box worker has invalid private pipe paths")
	}
	commandParentProcessID, err := strconv.ParseUint(arguments[5], 10, 32)
	if err != nil || uint32(commandParentProcessID) != parentProcessID {
		return E.New("sing-box worker has an invalid parent process ID")
	}
	return nil
}

func processCommandLine(process windows.Handle) ([]string, error) {
	var bufferLength uint32
	queryError := windows.NtQueryInformationProcess(
		process,
		windows.ProcessCommandLineInformation,
		nil,
		0,
		&bufferLength,
	)
	if bufferLength == 0 {
		if queryError != nil {
			return nil, queryError
		}
		return nil, E.New("named pipe client has an empty command line buffer")
	}
	buffer := make([]byte, bufferLength)
	queryError = windows.NtQueryInformationProcess(
		process,
		windows.ProcessCommandLineInformation,
		unsafe.Pointer(&buffer[0]),
		uint32(len(buffer)),
		&bufferLength,
	)
	if queryError != nil {
		return nil, queryError
	}
	commandLine := (*windows.NTUnicodeString)(unsafe.Pointer(&buffer[0]))
	if commandLine.Buffer == nil || commandLine.Length == 0 || commandLine.Length%2 != 0 {
		return nil, E.New("named pipe client has an invalid command line")
	}
	commandLineString := windows.UTF16ToString(unsafe.Slice(commandLine.Buffer, int(commandLine.Length/2)))
	return windows.DecomposeCommandLine(commandLineString)
}

func processParentIsApplication(process windows.Handle, expectedApplication windows.Handle) (bool, error) {
	parentProcessID, err := processParentID(process)
	if err != nil {
		return false, err
	}
	if parentProcessID == 0 {
		return false, nil
	}
	parentProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, parentProcessID)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, E.Cause(err, "open named pipe client parent")
	}
	defer windows.CloseHandle(parentProcess)
	parentImagePath, err := winioProcess.QueryFullProcessImageName(parentProcess, winioProcess.ImageNameFormatWin32Path)
	if err != nil {
		return false, E.Cause(err, "query named pipe client parent executable")
	}
	parentImage, err := openLockedExecutable(parentImagePath)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(parentImage)
	return sameWindowsFile(parentImage, expectedApplication)
}

func processParentID(process windows.Handle) (uint32, error) {
	var processInformation windows.PROCESS_BASIC_INFORMATION
	processInformationLength := uint32(unsafe.Sizeof(processInformation))
	err := windows.NtQueryInformationProcess(
		process,
		windows.ProcessBasicInformation,
		unsafe.Pointer(&processInformation),
		processInformationLength,
		&processInformationLength,
	)
	if err != nil {
		return 0, E.Cause(err, "query named pipe client parent")
	}
	parentProcessID := uint32(processInformation.InheritedFromUniqueProcessId)
	if parentProcessID == 0 || uintptr(parentProcessID) != processInformation.InheritedFromUniqueProcessId {
		return 0, nil
	}
	return parentProcessID, nil
}

func openLockedExecutable(path string) (windows.Handle, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(
		pathPointer,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_SEQUENTIAL_SCAN,
		0,
	)
}

func finalWindowsPath(file windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	for {
		length, err := windows.GetFinalPathNameByHandle(file, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", err
		}
		if length < uint32(len(buffer)) {
			return normalizeWindowsPath(windows.UTF16ToString(buffer[:length])), nil
		}
		buffer = make([]uint16, length+1)
	}
}

func normalizeWindowsPath(path string) string {
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return `\\` + path[len(`\\?\UNC\`):]
	}
	return strings.TrimPrefix(path, `\\?\`)
}

func sameWindowsFile(first windows.Handle, second windows.Handle) (bool, error) {
	var firstInformation windows.ByHandleFileInformation
	err := windows.GetFileInformationByHandle(first, &firstInformation)
	if err != nil {
		return false, E.Cause(err, "query named pipe client executable identity")
	}
	var secondInformation windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(second, &secondInformation)
	if err != nil {
		return false, E.Cause(err, "query installed application executable identity")
	}
	return firstInformation.VolumeSerialNumber == secondInformation.VolumeSerialNumber &&
		firstInformation.FileIndexHigh == secondInformation.FileIndexHigh &&
		firstInformation.FileIndexLow == secondInformation.FileIndexLow, nil
}

func (c *windowsAuthenticatedConnection) peerConnectionIdentity() peerIdentity {
	return c.identity
}

func (d *Daemon) registerPeerConnection(connection peerConnection) {
	d.peerAccess.Lock()
	defer d.peerAccess.Unlock()
	if d.peerConnections == nil {
		d.peerConnections = make(map[peerConnection]peerIdentity)
	}
	d.peerConnections[connection] = connection.peerConnectionIdentity()
}

func (d *Daemon) unregisterPeerConnection(connection peerConnection) {
	d.peerAccess.Lock()
	defer d.peerAccess.Unlock()
	delete(d.peerConnections, connection)
}

func (c *windowsAuthenticatedConnection) Close() error {
	c.close.Do(func() {
		c.daemon.unregisterPeerConnection(c)
		c.closeError = E.Errors(
			c.Conn.Close(),
			windows.CloseHandle(c.parentProcessImage),
			windows.CloseHandle(c.parentProcess),
			windows.CloseHandle(c.processImage),
			windows.CloseHandle(c.process),
		)
	})
	return c.closeError
}

var (
	_ credentials.TransportCredentials = (*windowsTransportCredentials)(nil)
	_ peerConnection                   = (*windowsAuthenticatedConnection)(nil)
)

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/tailscale/go-winio"
	winioProcess "github.com/tailscale/go-winio/pkg/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type windowsWorkerParent struct {
	process        windows.Handle
	processImage   windows.Handle
	executable     windows.Handle
	executablePath string
	signer         []byte
	userID         string
	sessionID      uint32
	pid            uint32
	exited         chan struct{}
	close          sync.Once
	closeError     error
}

type authenticatedWorkerListener struct {
	net.Listener
	parent *windowsWorkerParent
}

type windowsWorkerDaemonRelay struct {
	listener            net.Listener
	parent              *windowsWorkerParent
	onFailure           func(error)
	connections         map[net.Conn]struct{}
	connectionAccess    sync.Mutex
	connectionWaitGroup sync.WaitGroup
	closing             atomic.Bool
	close               sync.Once
	closeError          error
}

type windowsAuthenticatedDaemonConnection struct {
	net.Conn
	process      windows.Handle
	processImage windows.Handle
	close        sync.Once
	closeError   error
}

func prepareWorkerParent(parentProcessID uint32) (workerParent, error) {
	if os.Getppid() != int(parentProcessID) {
		return nil, E.New("worker was not started by the expected application process")
	}
	parentProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, parentProcessID)
	if err != nil {
		return nil, err
	}
	keepProcess := false
	defer func() {
		if !keepProcess {
			windows.CloseHandle(parentProcess)
		}
	}()
	identity, err := processIdentity(parentProcess, parentProcessID)
	if err != nil {
		return nil, err
	}
	parentImagePath, err := winioProcess.QueryFullProcessImageName(parentProcess, winioProcess.ImageNameFormatWin32Path)
	if err != nil {
		return nil, E.Cause(err, "query worker parent executable")
	}
	parentImage, err := openLockedExecutable(parentImagePath)
	if err != nil {
		return nil, err
	}
	keepParentImage := false
	defer func() {
		if !keepParentImage {
			windows.CloseHandle(parentImage)
		}
	}()
	workerExecutablePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	workerExecutable, err := openLockedExecutable(workerExecutablePath)
	if err != nil {
		return nil, err
	}
	keepWorkerExecutable := false
	defer func() {
		if !keepWorkerExecutable {
			windows.CloseHandle(workerExecutable)
		}
	}()
	workerFinalPath, err := finalWindowsPath(workerExecutable)
	if err != nil {
		return nil, err
	}
	_, expectedApplicationPath, err := installedApplicationPath(workerFinalPath)
	if err != nil {
		return nil, err
	}
	expectedApplication, err := openLockedExecutable(expectedApplicationPath)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(expectedApplication)
	parentFinalPath, err := finalWindowsPath(parentImage)
	if err != nil {
		return nil, err
	}
	expectedApplicationFinalPath, err := finalWindowsPath(expectedApplication)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(parentFinalPath, expectedApplicationFinalPath) {
		return nil, E.New("worker parent is not the installed sing-box application")
	}
	sameApplication, err := sameWindowsFile(parentImage, expectedApplication)
	if err != nil {
		return nil, err
	}
	if !sameApplication {
		return nil, E.New("worker parent executable was replaced")
	}
	err = validateApplicationProcessRole(parentProcess, expectedApplication)
	if err != nil {
		return nil, err
	}
	workerSigner, err := authenticodeSigner(workerFinalPath, workerExecutable)
	if err != nil {
		return nil, err
	}
	parentSigner, err := authenticodeSigner(parentFinalPath, parentImage)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(workerSigner, parentSigner) {
		return nil, E.New("worker and application have different signing certificates")
	}
	parentCreationTime, err := processCreationTime(parentProcess)
	if err != nil {
		return nil, err
	}
	workerCreationTime, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		return nil, err
	}
	if parentCreationTime >= workerCreationTime {
		return nil, E.New("worker parent was created after the worker process")
	}
	waitResult, err := windows.WaitForSingleObject(parentProcess, 0)
	if err != nil {
		return nil, err
	}
	if waitResult != uint32(windows.WAIT_TIMEOUT) {
		return nil, E.New("worker application parent exited during authentication")
	}
	parent := &windowsWorkerParent{
		process:        parentProcess,
		processImage:   parentImage,
		executable:     workerExecutable,
		executablePath: workerFinalPath,
		signer:         workerSigner,
		userID:         identity.UserID,
		sessionID:      identity.SessionID,
		pid:            parentProcessID,
		exited:         make(chan struct{}),
	}
	go func() {
		_, _ = windows.WaitForSingleObject(parent.process, windows.INFINITE)
		close(parent.exited)
	}()
	keepProcess = true
	keepParentImage = true
	keepWorkerExecutable = true
	return parent, nil
}

func listenWorkerEndpoint(path string, parent workerParent) (net.Listener, error) {
	windowsParent := parent.(*windowsWorkerParent)
	securityDescriptor := fmt.Sprintf(
		"D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;%s)",
		windowsParent.userID,
	)
	listener, err := winio.ListenPipe(path, &winio.PipeConfig{
		SecurityDescriptor: securityDescriptor,
		InputBufferSize:    pipeBufferSize,
		OutputBufferSize:   pipeBufferSize,
	})
	if err != nil {
		return nil, err
	}
	authenticatedListener := &authenticatedWorkerListener{Listener: listener, parent: windowsParent}
	go func() {
		<-windowsParent.exited
		authenticatedListener.Close()
	}()
	return authenticatedListener, nil
}

func (l *authenticatedWorkerListener) Accept() (net.Conn, error) {
	for {
		waitResult, err := windows.WaitForSingleObject(l.parent.process, 0)
		if err != nil {
			return nil, err
		}
		if waitResult != uint32(windows.WAIT_TIMEOUT) {
			return nil, E.New("worker application parent exited")
		}
		connection, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		descriptorConnection, loaded := connection.(fileDescriptorConnection)
		if !loaded {
			connection.Close()
			continue
		}
		var clientProcessID uint32
		err = windows.GetNamedPipeClientProcessId(windows.Handle(descriptorConnection.Fd()), &clientProcessID)
		if err != nil || clientProcessID != l.parent.pid {
			connection.Close()
			continue
		}
		waitResult, err = windows.WaitForSingleObject(l.parent.process, 0)
		if err != nil || waitResult != uint32(windows.WAIT_TIMEOUT) {
			connection.Close()
			return nil, E.New("worker application parent exited")
		}
		return connection, nil
	}
}

func (p *windowsWorkerParent) Close() error {
	p.close.Do(func() {
		p.closeError = E.Errors(
			windows.CloseHandle(p.executable),
			windows.CloseHandle(p.processImage),
			windows.CloseHandle(p.process),
		)
	})
	return p.closeError
}

func startWorkerDaemonRelay(path string, parent workerParent, onFailure func(error)) (io.Closer, error) {
	if path == "" {
		return nil, E.New("missing --daemon-relay-socket")
	}
	windowsParent := parent.(*windowsWorkerParent)
	listener, err := listenWorkerEndpoint(path, parent)
	if err != nil {
		return nil, err
	}
	relay := &windowsWorkerDaemonRelay{
		listener:    listener,
		parent:      windowsParent,
		onFailure:   onFailure,
		connections: make(map[net.Conn]struct{}),
	}
	go relay.serve()
	return relay, nil
}

func (r *windowsWorkerDaemonRelay) serve() {
	for {
		connection, err := r.listener.Accept()
		if err != nil {
			if !r.closing.Load() && !errors.Is(err, net.ErrClosed) {
				r.onFailure(E.Cause(err, "accept daemon relay connection"))
			}
			return
		}
		r.connectionAccess.Lock()
		if r.closing.Load() {
			r.connectionAccess.Unlock()
			connection.Close()
			return
		}
		r.connections[connection] = struct{}{}
		r.connectionWaitGroup.Add(1)
		r.connectionAccess.Unlock()
		go func() {
			r.relay(connection)
			r.connectionAccess.Lock()
			delete(r.connections, connection)
			r.connectionAccess.Unlock()
			r.connectionWaitGroup.Done()
		}()
	}
}

func (r *windowsWorkerDaemonRelay) relay(applicationConnection net.Conn) {
	daemonConnection, err := r.connectDaemon()
	if err != nil {
		applicationConnection.Close()
		return
	}
	copyCompleted := make(chan struct{}, 2)
	firstCopyCompleted := make(chan struct{})
	var firstCopy sync.Once
	copyConnection := func(destination io.Writer, source io.Reader) {
		_, _ = io.Copy(destination, source)
		firstCopy.Do(func() {
			close(firstCopyCompleted)
		})
		copyCompleted <- struct{}{}
	}
	go copyConnection(daemonConnection, applicationConnection)
	go copyConnection(applicationConnection, daemonConnection)
	select {
	case <-firstCopyCompleted:
	case <-r.parent.exited:
	}
	applicationConnection.Close()
	daemonConnection.Close()
	<-copyCompleted
	<-copyCompleted
}

func (r *windowsWorkerDaemonRelay) connectDaemon() (net.Conn, error) {
	connection, err := winio.DialPipe(daemonPipePath, nil)
	if err != nil {
		return nil, err
	}
	keepConnection := false
	defer func() {
		if !keepConnection {
			connection.Close()
		}
	}()
	descriptorConnection, loaded := connection.(fileDescriptorConnection)
	if !loaded {
		return nil, E.New("daemon endpoint is not a Windows named pipe")
	}
	var processID uint32
	err = windows.GetNamedPipeServerProcessId(windows.Handle(descriptorConnection.Fd()), &processID)
	if err != nil {
		return nil, E.Cause(err, "identify daemon named pipe server")
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, processID)
	if err != nil {
		return nil, E.Cause(err, "open daemon named pipe server process")
	}
	keepProcess := false
	defer func() {
		if !keepProcess {
			windows.CloseHandle(process)
		}
	}()
	err = validateDaemonProcessIdentity(processID)
	if err != nil {
		return nil, err
	}
	processImagePath, err := winioProcess.QueryFullProcessImageName(process, winioProcess.ImageNameFormatWin32Path)
	if err != nil {
		return nil, E.Cause(err, "query daemon named pipe server executable")
	}
	processImage, err := openLockedExecutable(processImagePath)
	if err != nil {
		return nil, E.Cause(err, "open daemon named pipe server executable")
	}
	keepProcessImage := false
	defer func() {
		if !keepProcessImage {
			windows.CloseHandle(processImage)
		}
	}()
	processImageFinalPath, err := finalWindowsPath(processImage)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(processImageFinalPath, r.parent.executablePath) {
		return nil, E.New("named pipe server is not the installed daemon")
	}
	sameExecutable, err := sameWindowsFile(processImage, r.parent.executable)
	if err != nil {
		return nil, err
	}
	if !sameExecutable {
		return nil, E.New("named pipe server daemon executable was replaced")
	}
	signer, err := authenticodeSigner(processImageFinalPath, processImage)
	if err != nil {
		return nil, E.Cause(err, "authenticate daemon named pipe server")
	}
	if !bytes.Equal(signer, r.parent.signer) {
		return nil, E.New("daemon server and worker have different signing certificates")
	}
	waitResult, err := windows.WaitForSingleObject(process, 0)
	if err != nil {
		return nil, err
	}
	if waitResult != uint32(windows.WAIT_TIMEOUT) {
		return nil, E.New("daemon named pipe server exited during authentication")
	}
	keepConnection = true
	keepProcess = true
	keepProcessImage = true
	return &windowsAuthenticatedDaemonConnection{
		Conn:         connection,
		process:      process,
		processImage: processImage,
	}, nil
}

func (r *windowsWorkerDaemonRelay) Close() error {
	r.close.Do(func() {
		r.closing.Store(true)
		r.closeError = r.listener.Close()
		r.connectionAccess.Lock()
		for connection := range r.connections {
			connection.Close()
		}
		r.connectionAccess.Unlock()
		r.connectionWaitGroup.Wait()
	})
	return r.closeError
}

func (c *windowsAuthenticatedDaemonConnection) Close() error {
	c.close.Do(func() {
		c.closeError = E.Errors(
			c.Conn.Close(),
			windows.CloseHandle(c.processImage),
			windows.CloseHandle(c.process),
		)
	})
	return c.closeError
}

func validateDaemonProcessIdentity(processID uint32) error {
	var sessionID uint32
	err := windows.ProcessIdToSessionId(processID, &sessionID)
	if err != nil {
		return E.Cause(err, "query daemon named pipe server session")
	}
	if sessionID != 0 {
		return E.New("daemon named pipe server is not in session zero")
	}
	managerHandle, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return E.Cause(err, "connect to service manager")
	}
	defer windows.CloseServiceHandle(managerHandle)
	serviceNamePointer, err := windows.UTF16PtrFromString(serviceName)
	if err != nil {
		return err
	}
	serviceHandle, err := windows.OpenService(
		managerHandle,
		serviceNamePointer,
		windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG,
	)
	if err != nil {
		return E.Cause(err, "open daemon service")
	}
	service := &mgr.Service{Name: serviceName, Handle: serviceHandle}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return E.Cause(err, "query daemon service status")
	}
	if status.State != svc.Running || status.ProcessId != processID {
		return E.New("named pipe server is not the running daemon service")
	}
	configuration, err := service.Config()
	if err != nil {
		return E.Cause(err, "query daemon service configuration")
	}
	if !strings.EqualFold(configuration.ServiceStartName, "LocalSystem") {
		return E.New("daemon service does not run as LocalSystem")
	}
	return nil
}

func processCreationTime(process windows.Handle) (int64, error) {
	var creationTime windows.Filetime
	var exitTime windows.Filetime
	var kernelTime windows.Filetime
	var userTime windows.Filetime
	err := windows.GetProcessTimes(process, &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return 0, err
	}
	return creationTime.Nanoseconds(), nil
}

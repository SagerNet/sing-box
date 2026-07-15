//go:build with_gvisor && windows

package tailssh

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/tailscale/util/winutil"
	"github.com/sagernet/tailscale/util/winutil/conpty"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

const (
	seAssignPrimaryToken = "SeAssignPrimaryTokenPrivilege"
	seIncreaseQuota      = "SeIncreaseQuotaPrivilege"
)

type windowsUserTokenProvider interface {
	AcquireWindowsUserToken(user *adapter.PlatformUser) (windows.Token, io.Closer, error)
}

func selectShellBackend(platformInterface adapter.PlatformInterface) shellBackend {
	backend := &windowsShellBackend{}
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		backend.userTokenProvider, _ = platformInterface.(windowsUserTokenProvider)
	}
	return backend
}

func CheckServerSupport(platformInterface adapter.PlatformInterface) (string, error) {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		_, loaded := platformInterface.(windowsUserTokenProvider)
		if !loaded {
			return "", E.New("platform shell does not provide Windows user tokens")
		}
		err := platformInterface.CheckPlatformShell()
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func lookupSFTPServer(platformInterface adapter.PlatformInterface) (string, error) {
	if platformInterface != nil && platformInterface.UsePlatformShell() {
		return platformInterface.LookupSFTPServer()
	}
	sftpPath, err := exec.LookPath("sftp-server")
	if err != nil {
		return "", E.New("sftp-server not found")
	}
	return sftpPath, nil
}

type windowsShellBackend struct {
	userTokenProvider windowsUserTokenProvider
}

func (b *windowsShellBackend) OpenSession(request shellRequest) (session shellSession, err error) {
	var (
		userToken    windows.Token
		userResource io.Closer
	)
	if b.userTokenProvider != nil {
		userToken, userResource, err = b.userTokenProvider.AcquireWindowsUserToken(request.User)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				err = E.Errors(err, userResource.Close())
			}
		}()
		userEnvironment, err := userToken.Environ(false)
		if err != nil {
			return nil, E.Cause(err, "query user environment")
		}
		request.Env = mergeWindowsEnvironment(userEnvironment, request.Env)
	}
	shell := request.User.Shell
	if request.Term != "" {
		session, err = openConPTYSession(request, shell, userToken)
		if err != nil && !errors.Is(err, conpty.ErrUnsupported) {
			return nil, err
		}
	}
	if request.Term == "" || err != nil {
		session, err = openPipeSession(request, shell, userToken)
		if err != nil {
			return nil, err
		}
	}
	if userResource != nil {
		session = &windowsUserShellSession{shellSession: session, resource: userResource}
	}
	return session, nil
}

func (b *windowsShellBackend) Close() error {
	return nil
}

func buildCommandLine(shell, command string) string {
	if command == "" {
		return `"` + shell + `"`
	}
	if isPowerShell(shell) {
		// -NoProfile/-NonInteractive keep the invoking user's PowerShell profile from
		// writing into the (binary) SFTP/stdout stream and corrupting it.
		return `"` + shell + `" -NoLogo -NoProfile -NonInteractive -Command ` + command
	}
	return `"` + shell + `" /c ` + command
}

func isPowerShell(shell string) bool {
	switch strings.ToLower(filepath.Base(shell)) {
	case "pwsh", "pwsh.exe", "powershell", "powershell.exe":
		return true
	default:
		return false
	}
}

func mergeWindowsEnvironment(baseEnvironment, overrideEnvironment []string) []string {
	environment := make([]string, 0, len(baseEnvironment)+len(overrideEnvironment))
	variableIndex := make(map[string]int, len(baseEnvironment)+len(overrideEnvironment))
	for _, variables := range [][]string{baseEnvironment, overrideEnvironment} {
		for _, variable := range variables {
			name := windowsEnvironmentName(variable)
			index, loaded := variableIndex[name]
			if loaded {
				environment[index] = variable
			} else {
				variableIndex[name] = len(environment)
				environment = append(environment, variable)
			}
		}
	}
	return environment
}

func windowsEnvironmentName(variable string) string {
	start := 0
	if strings.HasPrefix(variable, "=") {
		start = 1
	}
	separator := strings.IndexByte(variable[start:], '=')
	if separator == -1 {
		return strings.ToLower(variable)
	}
	return strings.ToLower(variable[:start+separator])
}

// clampConsoleDimension keeps a client-supplied window dimension within the
// positive int16 range expected by windows.Coord; values above 32767 would
// otherwise wrap negative and make ConPTY reject the size.
func clampConsoleDimension(value uint16) int16 {
	if value < 1 {
		return 1
	}
	if value > 0x7fff {
		return 0x7fff
	}
	return int16(value)
}

func createShellProcess(shell string, request shellRequest, startupInfo *windows.StartupInfo, inheritHandles bool, createProcessFlags uint32, userToken windows.Token) (windows.Handle, error) {
	cmdLine := buildCommandLine(shell, request.Command)
	cmdLine16, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, E.Cause(err, "encode command line")
	}
	exe16, err := windows.UTF16PtrFromString(shell)
	if err != nil {
		return 0, E.Cause(err, "encode shell path")
	}
	// Pass a nil lpCurrentDirectory for an empty HomeDir so the child inherits the
	// parent's working directory; a non-nil empty path makes CreateProcess fail.
	var dir16 *uint16
	if request.User.HomeDir != "" {
		dir16, err = windows.UTF16PtrFromString(request.User.HomeDir)
		if err != nil {
			return 0, E.Cause(err, "encode home directory")
		}
	}
	// NewEnvBlock requires the variables sorted case-insensitively by name.
	envCopy := slices.Clone(request.Env)
	slices.SortFunc(envCopy, func(a, b string) int {
		return strings.Compare(windowsEnvironmentName(a), windowsEnvironmentName(b))
	})
	envBlock := winutil.NewEnvBlock(envCopy)
	var processInfo windows.ProcessInformation
	if userToken == 0 {
		err = windows.CreateProcess(
			exe16,
			cmdLine16,
			nil,
			nil,
			inheritHandles,
			createProcessFlags|windows.CREATE_NEW_PROCESS_GROUP,
			envBlock,
			dir16,
			startupInfo,
			&processInfo,
		)
	} else {
		err = winio.RunWithPrivileges([]string{seAssignPrimaryToken, seIncreaseQuota}, func() error {
			return windows.CreateProcessAsUser(
				userToken,
				exe16,
				cmdLine16,
				nil,
				nil,
				inheritHandles,
				createProcessFlags|windows.CREATE_NEW_PROCESS_GROUP,
				envBlock,
				dir16,
				startupInfo,
				&processInfo,
			)
		})
	}
	if err != nil {
		return 0, E.Cause(err, "create process")
	}
	windows.CloseHandle(processInfo.Thread)
	return processInfo.Process, nil
}

type windowsUserShellSession struct {
	shellSession
	resource io.Closer
}

func (s *windowsUserShellSession) Close() error {
	return E.Errors(s.shellSession.Close(), s.resource.Close())
}

type conptyShellSession struct {
	console  *conpty.PseudoConsole
	input    io.WriteCloser
	output   io.ReadCloser
	process  windows.Handle
	done     chan struct{}
	exitCode uint32
}

func openConPTYSession(request shellRequest, shell string, userToken windows.Token) (shellSession, error) {
	cols := request.Cols
	rows := request.Rows
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	console, err := conpty.NewPseudoConsole(windows.Coord{X: clampConsoleDimension(cols), Y: clampConsoleDimension(rows)})
	if err != nil {
		if errors.Is(err, conpty.ErrUnsupported) {
			return nil, conpty.ErrUnsupported
		}
		return nil, E.Cause(err, "create pseudo console")
	}
	var startupInfoBuilder winutil.StartupInfoBuilder
	err = console.ConfigureStartupInfo(&startupInfoBuilder)
	if err != nil {
		console.Close()
		return nil, E.Cause(err, "configure startup info")
	}
	startupInfo, inheritHandles, createProcessFlags, err := startupInfoBuilder.Resolve()
	if err != nil {
		startupInfoBuilder.Close()
		console.Close()
		return nil, E.Cause(err, "resolve startup info")
	}
	process, err := createShellProcess(shell, request, startupInfo, inheritHandles, createProcessFlags, userToken)
	startupInfoBuilder.Close()
	if err != nil {
		console.Close()
		return nil, err
	}
	session := &conptyShellSession{
		console: console,
		input:   console.InputPipe(),
		output:  console.OutputPipe(),
		process: process,
		done:    make(chan struct{}),
	}
	go session.waitProcess()
	return session, nil
}

func (s *conptyShellSession) waitProcess() {
	windows.WaitForSingleObject(s.process, windows.INFINITE)
	windows.GetExitCodeProcess(s.process, &s.exitCode)
	// Close the pseudoconsole now that the child has exited so its output pipe reaches
	// EOF and the reader in pumpSession unblocks; without this the output pipe only
	// EOFs at handler teardown, hanging the session while the client stays connected.
	// PseudoConsole.Close is idempotent, so the later Close() in conptyShellSession.Close
	// is a safe no-op. The concurrent pumpSession output drain satisfies Close's
	// requirement that the output reader keep draining until EOF.
	s.console.Close()
	close(s.done)
}

func (s *conptyShellSession) Read(p []byte) (int, error) {
	n, err := s.output.Read(p)
	if errors.Is(err, os.ErrClosed) {
		return n, io.EOF
	}
	return n, err
}

func (s *conptyShellSession) Write(p []byte) (int, error) {
	return s.input.Write(p)
}

func (s *conptyShellSession) Resize(rows, cols uint16) error {
	return s.console.Resize(windows.Coord{X: clampConsoleDimension(cols), Y: clampConsoleDimension(rows)})
}

func (s *conptyShellSession) Signal(sig int) error {
	if s.process == 0 {
		return nil
	}
	switch sig {
	case 2: // SIGINT: deliver Ctrl-C through the pseudo console input
		_, err := s.input.Write([]byte{0x03})
		return err
	case 9, 15:
		return windows.TerminateProcess(s.process, 1)
	default:
		return nil
	}
}

func (s *conptyShellSession) CloseWrite() error {
	return s.input.Close()
}

func (s *conptyShellSession) Wait() (uint32, error) {
	<-s.done
	return s.exitCode, nil
}

func (s *conptyShellSession) Close() error {
	if s.process == 0 {
		return nil
	}
	select {
	case <-s.done:
	default:
		windows.TerminateProcess(s.process, 1)
		<-s.done
	}
	s.console.Close()
	windows.CloseHandle(s.process)
	s.process = 0
	return nil
}

type pipeShellSession struct {
	stdin    *os.File
	stdout   *os.File
	process  windows.Handle
	done     chan struct{}
	exitCode uint32
}

func openPipeSession(request shellRequest, shell string, userToken windows.Token) (shellSession, error) {
	var stdinR, stdinW windows.Handle
	err := windows.CreatePipe(&stdinR, &stdinW, nil, 0)
	if err != nil {
		return nil, E.Cause(err, "create stdin pipe")
	}
	var stdoutR, stdoutW windows.Handle
	err = windows.CreatePipe(&stdoutR, &stdoutW, nil, 0)
	if err != nil {
		windows.CloseHandle(stdinR)
		windows.CloseHandle(stdinW)
		return nil, E.Cause(err, "create stdout pipe")
	}
	// Give stderr its own handle: SetStdHandles takes ownership of each handle it
	// receives and StartupInfoBuilder.Close closes StdOutput and StdErr separately,
	// so passing stdoutW twice would CloseHandle the same value twice.
	var stderrW windows.Handle
	currentProcess := windows.CurrentProcess()
	err = windows.DuplicateHandle(currentProcess, stdoutW, currentProcess, &stderrW, 0, false, windows.DUPLICATE_SAME_ACCESS)
	if err != nil {
		windows.CloseHandle(stdinR)
		windows.CloseHandle(stdinW)
		windows.CloseHandle(stdoutR)
		windows.CloseHandle(stdoutW)
		return nil, E.Cause(err, "duplicate stderr handle")
	}
	var startupInfoBuilder winutil.StartupInfoBuilder
	err = startupInfoBuilder.SetStdHandles(stdinR, stdoutW, stderrW)
	if err != nil {
		windows.CloseHandle(stdinR)
		windows.CloseHandle(stdinW)
		windows.CloseHandle(stdoutR)
		windows.CloseHandle(stdoutW)
		windows.CloseHandle(stderrW)
		return nil, E.Cause(err, "set std handles")
	}
	startupInfo, inheritHandles, createProcessFlags, err := startupInfoBuilder.Resolve()
	if err != nil {
		startupInfoBuilder.Close()
		windows.CloseHandle(stdinW)
		windows.CloseHandle(stdoutR)
		return nil, E.Cause(err, "resolve startup info")
	}
	process, err := createShellProcess(shell, request, startupInfo, inheritHandles, createProcessFlags, userToken)
	startupInfoBuilder.Close()
	if err != nil {
		windows.CloseHandle(stdinW)
		windows.CloseHandle(stdoutR)
		return nil, err
	}
	session := &pipeShellSession{
		stdin:   os.NewFile(uintptr(stdinW), "pipe-stdin"),
		stdout:  os.NewFile(uintptr(stdoutR), "pipe-stdout"),
		process: process,
		done:    make(chan struct{}),
	}
	go session.waitProcess()
	return session, nil
}

func (s *pipeShellSession) waitProcess() {
	windows.WaitForSingleObject(s.process, windows.INFINITE)
	windows.GetExitCodeProcess(s.process, &s.exitCode)
	close(s.done)
}

func (s *pipeShellSession) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *pipeShellSession) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *pipeShellSession) Resize(_, _ uint16) error {
	return nil
}

func (s *pipeShellSession) Signal(sig int) error {
	if s.process == 0 {
		return nil
	}
	switch sig {
	case 9, 15:
		return windows.TerminateProcess(s.process, 1)
	default:
		return nil
	}
}

func (s *pipeShellSession) CloseWrite() error {
	return s.stdin.Close()
}

func (s *pipeShellSession) Wait() (uint32, error) {
	<-s.done
	return s.exitCode, nil
}

func (s *pipeShellSession) Close() error {
	if s.process == 0 {
		return nil
	}
	s.stdin.Close()
	select {
	case <-s.done:
	default:
		windows.TerminateProcess(s.process, 1)
		<-s.done
	}
	s.stdout.Close()
	windows.CloseHandle(s.process)
	s.process = 0
	return nil
}

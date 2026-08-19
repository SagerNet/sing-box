//go:build with_gvisor && !windows

package tailssh

import (
	"os"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
)

type platformShellBackend struct {
	platform adapter.PlatformInterface
}

func (b *platformShellBackend) OpenSession(request shellRequest) (shellSession, error) {
	session, err := b.platform.OpenShellSession(request.User, request.Command, request.Env, request.Term, int32(request.Rows), int32(request.Cols))
	if err != nil {
		return nil, err
	}
	dupFd, err := syscall.Dup(int(session.MasterFD()))
	if err != nil {
		session.Close()
		return nil, err
	}
	master := os.NewFile(uintptr(dupFd), "pty-master")
	shellSession := &platformShellSession{
		session: session,
		master:  master,
		isPty:   request.Term != "",
	}
	if shellSession.isPty && (request.WidthPixels > 0 || request.HeightPixels > 0) {
		_ = SetWinsize(int(master.Fd()), request.Rows, request.Cols, request.WidthPixels, request.HeightPixels)
	}
	return shellSession, nil
}

func (b *platformShellBackend) Close() error {
	return nil
}

type platformShellSession struct {
	session adapter.ShellSession
	master  *os.File
	isPty   bool
}

func (s *platformShellSession) Read(p []byte) (int, error) {
	return s.master.Read(p)
}

func (s *platformShellSession) Write(p []byte) (int, error) {
	return s.master.Write(p)
}

func (s *platformShellSession) Close() error {
	return common.Close(s.master, s.session)
}

func (s *platformShellSession) CloseWrite() error {
	if s.isPty {
		return nil
	}
	return syscall.Shutdown(int(s.master.Fd()), syscall.SHUT_WR)
}

func (s *platformShellSession) Resize(rows, cols, widthPixels, heightPixels uint16) error {
	err := s.session.Resize(int32(rows), int32(cols))
	if err != nil {
		return err
	}
	// The platform interface carries no pixel dimensions; set them directly
	// on the duplicated pty master.
	if s.isPty && (widthPixels > 0 || heightPixels > 0) {
		return SetWinsize(int(s.master.Fd()), rows, cols, widthPixels, heightPixels)
	}
	return nil
}

func (s *platformShellSession) Signal(sig int) error {
	return s.session.Signal(int32(sig))
}

func (s *platformShellSession) Wait() (uint32, error) {
	exitStatus, err := s.session.WaitExit()
	if err != nil {
		return 0, err
	}
	return uint32(exitStatus), nil
}

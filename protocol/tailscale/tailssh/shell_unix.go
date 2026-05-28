//go:build unix && !ios

package tailssh

import (
	"os"
	"syscall"
)

type Shell struct {
	master *os.File
	waiter *ProcessWaiter
	isPty  bool
}

func OpenPtyShell(shell string, args, env []string, dir string, uid, gid int, groups []int, rows, cols uint16) (*Shell, error) {
	master, process, err := StartPtyProcess(shell, args, env, dir, uid, gid, groups, rows, cols)
	if err != nil {
		return nil, err
	}
	return &Shell{
		master: master,
		waiter: NewProcessWaiter(process),
		isPty:  true,
	}, nil
}

func OpenSocketpairShell(shell string, args, env []string, dir string, uid, gid int, groups []int) (*Shell, error) {
	master, process, err := StartSocketpairProcess(shell, args, env, dir, uid, gid, groups)
	if err != nil {
		return nil, err
	}
	return &Shell{
		master: master,
		waiter: NewProcessWaiter(process),
	}, nil
}

func (s *Shell) MasterFD() int {
	return int(s.master.Fd())
}

func (s *Shell) IsPty() bool {
	return s.isPty
}

func (s *Shell) Read(p []byte) (int, error) {
	return s.master.Read(p)
}

func (s *Shell) Write(p []byte) (int, error) {
	return s.master.Write(p)
}

func (s *Shell) Resize(rows, cols uint16) error {
	if !s.isPty {
		return nil
	}
	return SetWinsize(int(s.master.Fd()), rows, cols)
}

func (s *Shell) Signal(sig int) error {
	return s.waiter.Signal(sig)
}

func (s *Shell) CloseWrite() error {
	if s.isPty {
		// A pty has no half-close; stdin EOF is delivered via the line discipline.
		return nil
	}
	// The socketpair is a single SOCK_STREAM used for both directions; shutting
	// down the write side delivers EOF to the child without killing it.
	return syscall.Shutdown(int(s.master.Fd()), syscall.SHUT_WR)
}

func (s *Shell) Wait() (uint32, error) {
	return s.waiter.Wait()
}

func (s *Shell) Close() error {
	// Skip the kill once the child has been reaped: its PID may already have been
	// reused, and Kill(-pid) would then signal an unrelated process group.
	if !s.waiter.Exited() {
		syscall.Kill(-s.waiter.Pid(), syscall.SIGKILL)
	}
	s.master.Close()
	return nil
}

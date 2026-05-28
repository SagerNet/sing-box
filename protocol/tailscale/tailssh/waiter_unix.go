//go:build unix

package tailssh

import (
	"os"
	"syscall"
)

type ProcessWaiter struct {
	process *os.Process
	state   *os.ProcessState
	waitErr error
	done    chan struct{}
}

func NewProcessWaiter(process *os.Process) *ProcessWaiter {
	pw := &ProcessWaiter{
		process: process,
		done:    make(chan struct{}),
	}
	go func() {
		pw.state, pw.waitErr = pw.process.Wait()
		close(pw.done)
	}()
	return pw
}

func (pw *ProcessWaiter) Wait() (uint32, error) {
	<-pw.done
	if pw.waitErr != nil {
		return 0, pw.waitErr
	}
	status, loaded := pw.state.Sys().(syscall.WaitStatus)
	if !loaded {
		if pw.state.Success() {
			return 0, nil
		}
		return 1, nil
	}
	if status.Signaled() {
		return uint32(128 + status.Signal()), nil
	}
	return uint32(status.ExitStatus()), nil
}

func (pw *ProcessWaiter) Exited() bool {
	select {
	case <-pw.done:
		return true
	default:
		return false
	}
}

func (pw *ProcessWaiter) Signal(sig int) error {
	return pw.process.Signal(syscall.Signal(sig))
}

func (pw *ProcessWaiter) Pid() int {
	return pw.process.Pid
}

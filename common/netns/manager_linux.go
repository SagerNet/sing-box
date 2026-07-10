package netns

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/netlink"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	vnetns "github.com/vishvananda/netns"
)

type holder struct {
	command    *exec.Cmd
	pipeWriter *os.File
	pidFile    string
}

func (m *Manager) start() error {
	for _, namespace := range m.namespaces {
		switch namespace.Type {
		case C.NetNsTypeDefault:
			path := namespace.DefaultOptions.Path
			if !strings.HasPrefix(path, "/") {
				path = "/run/netns/" + path
			}
			_, err := os.Stat(path)
			if err != nil {
				return E.Cause(err, "network namespace[", namespace.Tag, "]")
			}
		case C.NetNsTypeUnshare:
			err := m.startNamespace(namespace)
			if err != nil {
				return E.Cause(err, "network namespace[", namespace.Tag, "]")
			}
		}
	}
	return nil
}

func (m *Manager) startNamespace(namespace option.NetworkNamespace) error {
	if len(m.holderArgs) == 0 {
		return E.New("unshare network namespace is only supported in `sing-box run`")
	}
	created, err := m.startHolder(namespace.UnshareOptions.PidFile)
	if err != nil {
		return err
	}
	m.holders = append(m.holders, created)
	pid := created.command.Process.Pid
	if created.pidFile != "" {
		err = os.WriteFile(created.pidFile, []byte(strconv.Itoa(pid)+"\n"), 0o644)
		if err != nil {
			return E.Cause(err, "write pid file")
		}
	}
	m.paths[namespace.Tag] = netnsPath(pid)
	m.logger.Info("created network namespace[", namespace.Tag, "], holder pid: ", pid)
	if os.Geteuid() == 0 {
		m.logger.Info("enter network namespace[", namespace.Tag, "] with: nsenter -n -t ", pid)
	} else {
		m.logger.Info("enter network namespace[", namespace.Tag, "] with: nsenter -U --preserve-credentials -n -t ", pid)
	}
	return nil
}

func (m *Manager) close() error {
	for _, created := range m.holders {
		created.pipeWriter.Close()
		if created.pidFile != "" {
			os.Remove(created.pidFile)
		}
	}
	m.holders = nil
	return nil
}

func netnsPath(pid int) string {
	return "/proc/" + strconv.Itoa(pid) + "/ns/net"
}

func (m *Manager) startHolder(pidFile string) (*holder, error) {
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	command := exec.Command(m.holderArgs[0], m.holderArgs[1:]...)
	command.Stdin = pipeReader
	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNET,
	}
	err = command.Start()
	pipeReader.Close()
	if err != nil {
		pipeWriter.Close()
		return nil, E.Cause(err, "start holder process")
	}
	go command.Wait()
	err = setupNamespace(command.Process.Pid)
	if err != nil {
		pipeWriter.Close()
		return nil, err
	}
	return &holder{command: command, pipeWriter: pipeWriter, pidFile: pidFile}, nil
}

func setupNamespace(pid int) error {
	resultChannel := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		resultChannel <- setupNamespaceThread(pid)
	}()
	return <-resultChannel
}

func setupNamespaceThread(pid int) error {
	targetNs, err := vnetns.GetFromPath(netnsPath(pid))
	if err != nil {
		return E.Cause(err, "open created netns")
	}
	defer targetNs.Close()
	err = vnetns.Set(targetNs)
	if err != nil {
		return E.Cause(err, "enter created netns")
	}
	loopbackLink, err := netlink.LinkByName("lo")
	if err != nil {
		return E.Cause(err, "find lo")
	}
	err = netlink.LinkSetUp(loopbackLink)
	if err != nil {
		return E.Cause(err, "set lo up")
	}
	return nil
}

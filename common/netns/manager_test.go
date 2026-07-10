//go:build linux

package netns

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
)

func TestUnshareNamespace(t *testing.T) {
	if os.Getenv("NETNS_TEST_HOLDER") == "1" {
		Hold()
	}
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipeReader.Close()
	defer pipeWriter.Close()
	os.Setenv("NETNS_TEST_HOLDER", "1")
	defer os.Unsetenv("NETNS_TEST_HOLDER")
	manager, err := NewManager(logger.NOP(), []option.NetworkNamespace{{
		Type: C.NetNsTypeUnshare,
		Tag:  "test",
		UnshareOptions: option.UnshareNetworkNamespaceOptions{
			PidFile: "/proc/self/fd/" + F.ToString(pipeWriter.Fd()),
		},
	}}, []string{"/proc/self/exe", "-test.run=^TestUnshareNamespace$"})
	if err != nil {
		t.Fatal(err)
	}
	err = manager.Start(adapter.StartStateInitialize)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	pipeReader.SetReadDeadline(time.Now().Add(10 * time.Second))
	pidLine, err := bufio.NewReader(pipeReader).ReadString('\n')
	if err != nil {
		t.Fatal("read pid from pipe: ", err)
	}
	pid, err := strconv.Atoi(strings.TrimSuffix(pidLine, "\n"))
	if err != nil {
		t.Fatal("parse pid: ", err)
	}

	resolvedPath := manager.ResolvePath("test")
	if resolvedPath != netnsPath(pid) {
		t.Fatal("resolved path ", resolvedPath, " does not match pid ", pid)
	}
	currentNs, err := os.Readlink("/proc/thread-self/ns/net")
	if err != nil {
		t.Fatal(err)
	}
	holderNs, err := os.Readlink(resolvedPath)
	if err != nil {
		t.Fatal("holder netns not accessible: ", err)
	}
	if currentNs == holderNs {
		t.Fatal("holder is in the current netns")
	}

	err = manager.Close()
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		_, err = os.Stat("/proc/" + strconv.Itoa(pid))
		if err != nil {
			return
		}
	}
	t.Fatal("holder process did not exit after close")
}

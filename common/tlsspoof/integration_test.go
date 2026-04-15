//go:build linux || darwin

package tlsspoof

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("integration test requires root")
	}
}

func tcpdumpObserver(t *testing.T, iface string, port uint16, needle string, do func(), wait time.Duration) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tcpdump", "-i", iface, "-n", "-A", "-l",
		"-s", "4096", fmt.Sprintf("tcp and port %d", port))
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt)
	}
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	})

	ready := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "listening on") {
				close(ready)
				io.Copy(io.Discard, stderr)
				return
			}
		}
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("tcpdump did not attach within 2s")
	}

	var found atomic.Bool
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), needle) {
				found.Store(true)
			}
		}
	}()

	do()

	time.Sleep(200 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	<-readerDone
	return found.Load()
}

func dialLocalEchoServer(t *testing.T) (client net.Conn, serverPort uint16) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := listener.Accept()
		if err == nil {
			accepted <- c
		}
		close(accepted)
	}()
	addr := listener.Addr().(*net.TCPAddr)
	client, err = net.Dial("tcp4", addr.String())
	require.NoError(t, err)
	server := <-accepted
	require.NotNil(t, server)

	go io.Copy(io.Discard, server)
	t.Cleanup(func() {
		client.Close()
		server.Close()
		listener.Close()
	})
	return client, uint16(addr.Port)
}

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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration test requires root; re-run with `go test -exec sudo`")
	}
}

func tcpdumpObserver(t *testing.T, iface string, port uint16, needle string, do func(), wait time.Duration) bool {
	t.Helper()
	return tcpdumpObserverMulti(t, iface, port, []string{needle}, do, wait)[needle]
}

// tcpdumpObserverMulti captures tcpdump output while do() executes and reports
// which of the provided needles were observed in the raw ASCII dump. Use this
// to assert that distinct payloads (e.g. fake vs real ClientHello) are both on
// the wire.
func tcpdumpObserverMulti(t *testing.T, iface string, port uint16, needles []string, do func(), wait time.Duration) map[string]bool {
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

	var access sync.Mutex
	found := make(map[string]bool, len(needles))
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			access.Lock()
			for _, needle := range needles {
				if !found[needle] && strings.Contains(line, needle) {
					found[needle] = true
				}
			}
			access.Unlock()
		}
	}()

	do()

	time.Sleep(200 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	<-readerDone
	access.Lock()
	defer access.Unlock()
	result := make(map[string]bool, len(needles))
	for _, needle := range needles {
		result[needle] = found[needle]
	}
	return result
}

func dialLocalEchoServer(t *testing.T) (client net.Conn, serverPort uint16) {
	return dialLocalEchoServerFamily(t, "tcp4", "127.0.0.1:0")
}

func dialLocalEchoServerIPv6(t *testing.T) (client net.Conn, serverPort uint16) {
	return dialLocalEchoServerFamily(t, "tcp6", "[::1]:0")
}

func dialLocalEchoServerFamily(t *testing.T, network, address string) (client net.Conn, serverPort uint16) {
	t.Helper()
	listener, err := net.Listen(network, address)
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
	client, err = net.Dial(network, addr.String())
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

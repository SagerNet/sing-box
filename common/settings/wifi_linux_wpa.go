package settings

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
)

var wpaSocketCounter atomic.Uint64

type wpaSupplicantMonitor struct {
	socketPath  string
	callback    func(adapter.WIFIState)
	cancel      context.CancelFunc
	monitorConn *net.UnixConn
	connMutex   sync.Mutex
}

func newWpaSupplicantMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	socketDirs := []string{"/var/run/wpa_supplicant", "/run/wpa_supplicant"}
	for _, socketDir := range socketDirs {
		entries, err := os.ReadDir(socketDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "." || entry.Name() == ".." {
				continue
			}
			socketPath := filepath.Join(socketDir, entry.Name())
			id := wpaSocketCounter.Add(1)
			localAddr := &net.UnixAddr{Name: fmt.Sprintf("@sing-box-wpa-%d-%d", os.Getpid(), id), Net: "unixgram"}
			remoteAddr := &net.UnixAddr{Name: socketPath, Net: "unixgram"}
			conn, err := net.DialUnix("unixgram", localAddr, remoteAddr)
			if err != nil {
				continue
			}
			conn.Close()
			return &wpaSupplicantMonitor{socketPath: socketPath, callback: callback}, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *wpaSupplicantMonitor) ReadWIFIState() adapter.WIFIState {
	id := wpaSocketCounter.Add(1)
	localAddr := &net.UnixAddr{Name: fmt.Sprintf("@sing-box-wpa-%d-%d", os.Getpid(), id), Net: "unixgram"}
	remoteAddr := &net.UnixAddr{Name: m.socketPath, Net: "unixgram"}
	conn, err := net.DialUnix("unixgram", localAddr, remoteAddr)
	if err != nil {
		return adapter.WIFIState{}
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(3 * time.Second))

	status, err := m.sendCommand(conn, "STATUS")
	if err != nil {
		return adapter.WIFIState{}
	}

	var ssid, bssid string
	var connected bool
	scanner := bufio.NewScanner(strings.NewReader(status))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "wpa_state=") {
			state := strings.TrimPrefix(line, "wpa_state=")
			connected = state == "COMPLETED"
		} else if strings.HasPrefix(line, "ssid=") {
			ssid = strings.TrimPrefix(line, "ssid=")
		} else if strings.HasPrefix(line, "bssid=") {
			bssid = strings.TrimPrefix(line, "bssid=")
		}
	}

	if !connected || ssid == "" {
		return adapter.WIFIState{}
	}

	return adapter.WIFIState{
		SSID:  ssid,
		BSSID: strings.ToUpper(strings.ReplaceAll(bssid, ":", "")),
	}
}

// sendCommand sends a command to wpa_supplicant and returns the response.
// Commands are sent without trailing newlines per the wpa_supplicant control
// interface protocol - the official wpa_ctrl.c sends raw command strings.
func (m *wpaSupplicantMonitor) sendCommand(conn *net.UnixConn, command string) (string, error) {
	_, err := conn.Write([]byte(command))
	if err != nil {
		return "", err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	response := string(buf[:n])
	if strings.HasPrefix(response, "FAIL") {
		return "", os.ErrInvalid
	}

	return strings.TrimSpace(response), nil
}

func (m *wpaSupplicantMonitor) Start() error {
	if m.callback == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	state := m.ReadWIFIState()
	go m.monitorEvents(ctx, state)
	m.callback(state)

	return nil
}

func (m *wpaSupplicantMonitor) monitorEvents(ctx context.Context, lastState adapter.WIFIState) {
	var consecutiveErrors int
	var debounceTimer *time.Timer
	var debounceMutex sync.Mutex

	localAddr := &net.UnixAddr{Name: fmt.Sprintf("@sing-box-wpa-mon-%d", os.Getpid()), Net: "unixgram"}
	remoteAddr := &net.UnixAddr{Name: m.socketPath, Net: "unixgram"}
	conn, err := net.DialUnix("unixgram", localAddr, remoteAddr)
	if err != nil {
		return
	}
	defer conn.Close()

	m.connMutex.Lock()
	m.monitorConn = conn
	m.connMutex.Unlock()

	// ATTACH/DETACH commands use os_strcmp() for exact matching in wpa_supplicant,
	// so they must be sent without trailing newlines.
	// See: https://w1.fi/cgit/hostap/tree/wpa_supplicant/ctrl_iface_unix.c
	_, err = conn.Write([]byte("ATTACH"))
	if err != nil {
		return
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || !strings.HasPrefix(string(buf[:n]), "OK") {
		return
	}

	for {
		select {
		case <-ctx.Done():
			debounceMutex.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceMutex.Unlock()
			conn.Write([]byte("DETACH"))
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			consecutiveErrors++
			if consecutiveErrors > 10 {
				return
			}
			time.Sleep(time.Second)
			continue
		}
		consecutiveErrors = 0

		msg := string(buf[:n])
		if strings.Contains(msg, "CTRL-EVENT-CONNECTED") || strings.Contains(msg, "CTRL-EVENT-DISCONNECTED") {
			debounceMutex.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				state := m.ReadWIFIState()
				if state != lastState {
					lastState = state
					m.callback(state)
				}
			})
			debounceMutex.Unlock()
		}
	}
}

func (m *wpaSupplicantMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.connMutex.Lock()
	if m.monitorConn != nil {
		m.monitorConn.Close()
	}
	m.connMutex.Unlock()
	return nil
}

//go:build !windows

package geph

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGephProcessArgsAndFraming(t *testing.T) {
	controlAddress := freeControlAddress(t)
	requestLog := filepath.Join(t.TempDir(), "control-requests.jsonl")
	helper := writeGephHelper(t, controlAddress, []string{"Connected"}, requestLog)
	p := newGephProcess(context.Background(), helper, "/tmp/geph5.yaml", controlAddress, []string{"--extra-test-arg"}, time.Second)
	if got := p.args(); len(got) != 4 || got[0] != "--config" || got[1] != "/tmp/geph5.yaml" || got[2] != "--stdio-vpn" || got[3] != "--extra-test-arg" {
		t.Fatalf("unexpected arguments: %#v", got)
	}
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	requestLines, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatal(err)
	}
	request := strings.Split(strings.TrimSpace(string(requestLines)), "\n")[0]
	var requestBody struct {
		JSONRPC string   `json:"jsonrpc"`
		Method  string   `json:"method"`
		ID      string   `json:"id"`
		Params  []string `json:"params"`
	}
	if err := json.Unmarshal([]byte(request), &requestBody); err != nil {
		t.Fatalf("invalid conn_info request: %v", err)
	}
	if requestBody.JSONRPC != "2.0" || requestBody.Method != "conn_info" || requestBody.ID != gephReadinessRequestID || len(requestBody.Params) != 0 {
		t.Fatalf("unexpected conn_info request: %#v", requestBody)
	}

	packet := []byte{0x45, 0, 1, 2, 3}
	if err := p.sendPacket(packet); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-p.incoming:
		if string(got) != string(packet) {
			t.Fatalf("packet mismatch: %x", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for framed packet")
	}
}

func TestGephProcessWaitsForConnectedState(t *testing.T) {
	controlAddress := freeControlAddress(t)
	requestLog := filepath.Join(t.TempDir(), "control-requests.jsonl")
	helper := writeGephHelper(t, controlAddress, []string{"Connecting", "Connected"}, requestLog)
	p := newGephProcess(context.Background(), helper, "/tmp/geph5.yaml", controlAddress, nil, 2*time.Second)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	requestLines, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatal(err)
	}
	if requests := strings.Count(strings.TrimSpace(string(requestLines)), "\n") + 1; requests < 2 {
		t.Fatalf("expected at least two control requests, got %d", requests)
	}
}

func TestGephProcessStartupTimeout(t *testing.T) {
	controlAddress := freeControlAddress(t)
	helper := writeGephHelper(t, controlAddress, []string{"Connecting"}, "")
	p := newGephProcess(context.Background(), helper, "/tmp/geph5.yaml", controlAddress, nil, 400*time.Millisecond)
	err := p.Start()
	if err == nil {
		defer p.Close()
		t.Fatal("expected startup timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	select {
	case <-p.waitDone:
	default:
		t.Fatal("Geph child was not reaped after startup timeout")
	}
}

func TestGephProcessContextCancellation(t *testing.T) {
	controlAddress := freeControlAddress(t)
	helper := writeGephHelper(t, controlAddress, []string{"Connecting"}, "")
	ctx, cancel := context.WithCancel(context.Background())
	p := newGephProcess(ctx, helper, "/tmp/geph5.yaml", controlAddress, nil, 5*time.Second)
	time.AfterFunc(150*time.Millisecond, cancel)
	err := p.Start()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got: %v", err)
	}
	select {
	case <-p.waitDone:
	default:
		t.Fatal("Geph child was not reaped after context cancellation")
	}
}

func TestGephProcessRejectsOccupiedControlAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	p := newGephProcess(context.Background(), "/bin/false", "/tmp/geph5.yaml", listener.Addr().String(), nil, time.Second)
	err = p.Start()
	if err == nil || !strings.Contains(err.Error(), "already in use or unavailable") {
		t.Fatalf("expected occupied-address error, got: %v", err)
	}
}

func TestGephProcessRejectsInvalidRPCResponse(t *testing.T) {
	for name, response := range map[string]string{
		"version":        `{"jsonrpc":"1.0","id":"sing-box-geph-readiness","result":{"state":"Connected"}}` + "\n",
		"id":             `{"jsonrpc":"2.0","id":"somebody-else","result":{"state":"Connected"}}` + "\n",
		"malformed":      "{not-json}\n",
		"rpc error":      `{"jsonrpc":"2.0","id":"sing-box-geph-readiness","error":{"code":-32000,"message":"not ready"}}` + "\n",
		"missing result": `{"jsonrpc":"2.0","id":"sing-box-geph-readiness"}` + "\n",
		"oversized":      strings.Repeat("x", maxControlRPCResponseSize+1) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			controlAddress, waitServer := startOneShotControlServer(t, response)
			p := newGephProcess(context.Background(), "/bin/false", "/tmp/geph5.yaml", controlAddress, nil, time.Second)
			ready, _, err := p.queryConnectionState(context.Background())
			waitServer()
			if ready || err == nil {
				t.Fatalf("expected invalid response error, ready=%v err=%v", ready, err)
			}
			var protocolErr *gephControlProtocolError
			if !errors.As(err, &protocolErr) {
				t.Fatalf("expected protocol error, got: %v", err)
			}
		})
	}
}

func TestGephProcessErrorOnExitedProcess(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "geph5-helper.py")
	script := "#!/usr/bin/env python3\nimport sys\nsys.stderr.write('geph bootstrap failed')\nsys.exit(3)\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	p := newGephProcess(context.Background(), helper, "/tmp/geph5.yaml", freeControlAddress(t), nil, 500*time.Millisecond)
	err := p.Start()
	if err == nil {
		t.Fatal("expected startup failure")
	}
	if !strings.Contains(err.Error(), "process exited during startup") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "geph bootstrap failed") {
		t.Fatalf("missing stderr diagnostics: %v", err)
	}
}

func TestGephProcessRejectsOversizedPacket(t *testing.T) {
	p := newGephProcess(context.Background(), "/bin/false", "/tmp/geph5.yaml", "127.0.0.1:1080", nil, time.Second)
	if err := p.sendPacket(make([]byte, 65536)); err == nil {
		t.Fatal("expected oversized packet error")
	}
}

func TestGephFramingIsBigEndian(t *testing.T) {
	var header [2]byte
	binary.BigEndian.PutUint16(header[:], 0x1234)
	if header != [2]byte{0x12, 0x34} {
		t.Fatalf("unexpected framing: %x", header)
	}
}

func freeControlAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func writeGephHelper(t *testing.T, controlAddress string, states []string, requestLog string) string {
	t.Helper()
	host, port, err := net.SplitHostPort(controlAddress)
	if err != nil {
		t.Fatal(err)
	}
	statesJSON, err := json.Marshal(states)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "geph5-helper.py")
	script := fmt.Sprintf(`#!/usr/bin/env python3
import json
import socket
import struct
import sys
import threading

CONTROL_HOST = %q
CONTROL_PORT = %s
STATES = %s
REQUEST_LOG = %q

def serve_control():
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind((CONTROL_HOST, CONTROL_PORT))
    listener.listen()
    index = 0
    while True:
        conn, _ = listener.accept()
        with conn:
            reader = conn.makefile("rb")
            request_line = reader.readline()
            if not request_line:
                continue
            request = json.loads(request_line)
            if REQUEST_LOG:
                with open(REQUEST_LOG, "ab") as request_file:
                    request_file.write(request_line)
            state = STATES[min(index, len(STATES) - 1)]
            index += 1
            response = {
                "jsonrpc": "2.0",
                "id": request.get("id"),
                "result": {"state": state},
            }
            conn.sendall(json.dumps(response).encode() + b"\n")

threading.Thread(target=serve_control, daemon=True).start()

while True:
    header = sys.stdin.buffer.read(2)
    if len(header) != 2:
        break
    length = struct.unpack(">H", header)[0]
    packet = sys.stdin.buffer.read(length)
    if len(packet) != length:
        break
    sys.stdout.buffer.write(header + packet)
    sys.stdout.buffer.flush()
`, host, port, statesJSON, requestLog)
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return helper
}

func startOneShotControlServer(t *testing.T, response string) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if _, err = bufio.NewReader(conn).ReadString('\n'); err == nil {
			_, err = conn.Write([]byte(response))
		}
		done <- err
	}()
	return listener.Addr().String(), func() {
		t.Helper()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for control server")
		}
	}
}

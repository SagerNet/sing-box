package geph

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	defaultGephStartupTimeout = 15 * time.Second
	controlRPCPollInterval    = 100 * time.Millisecond
	controlRPCTimeout         = 500 * time.Millisecond
	maxControlRPCResponseSize = 64 * 1024
	maxStderrTailSize         = 8192
)

const gephReadinessRequestID = "sing-box-geph-readiness"

type gephProcess struct {
	ctx                                context.Context
	executable, config, controlAddress string
	extraArgs                          []string
	timeout                            time.Duration
	incoming                           chan []byte
	outgoing                           chan []byte
	done, waitDone                     chan struct{}
	closeOnce                          sync.Once
	stateMu                            sync.Mutex
	closed                             bool
	cmd                                *exec.Cmd
	stdin                              io.WriteCloser
	waitErr                            error
	stderrTail                         boundedStringBuffer
}

type gephConnInfoRequest struct {
	JSONRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      string   `json:"id"`
}

type gephConnInfoResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  *struct {
		State string `json:"state"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type gephControlProtocolError struct {
	err error
}

func (e *gephControlProtocolError) Error() string {
	return e.err.Error()
}

func (e *gephControlProtocolError) Unwrap() error {
	return e.err
}

func newGephControlProtocolError(format string, args ...any) error {
	return &gephControlProtocolError{err: fmt.Errorf(format, args...)}
}

type boundedStringBuffer struct {
	mu      sync.Mutex
	content []byte
	limit   int
}

func newBoundedStringBuffer(limit int) boundedStringBuffer {
	return boundedStringBuffer{limit: limit}
}

func (b *boundedStringBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limit > 0 && len(p) >= b.limit {
		b.content = append(b.content[:0], p[len(p)-b.limit:]...)
		return len(p), nil
	}
	if b.limit > 0 {
		excess := len(b.content) + len(p) - b.limit
		if excess > 0 {
			b.content = b.content[excess:]
		}
	}
	b.content = append(b.content, p...)
	return len(p), nil
}

func (b *boundedStringBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.content))
}

func newGephProcess(ctx context.Context, executable, config, controlAddress string, extraArgs []string, timeout time.Duration) *gephProcess {
	return &gephProcess{
		ctx:            ctx,
		executable:     executable,
		config:         config,
		controlAddress: controlAddress,
		extraArgs:      append([]string(nil), extraArgs...),
		timeout:        timeout,
		incoming:       make(chan []byte, 256),
		outgoing:       make(chan []byte, 256),
		done:           make(chan struct{}),
		waitDone:       make(chan struct{}),
		stderrTail:     newBoundedStringBuffer(maxStderrTailSize),
	}
}

func (p *gephProcess) args() []string {
	return append([]string{"--config", p.config, "--stdio-vpn"}, p.extraArgs...)
}

func (p *gephProcess) Start() error {
	if p.timeout <= 0 {
		p.timeout = defaultGephStartupTimeout
	}
	startupCtx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()

	if err := p.ensureControlAddressAvailable(startupCtx); err != nil {
		if startupCtx.Err() != nil {
			return p.startupContextError(startupCtx, nil)
		}
		return err
	}

	cmd := exec.CommandContext(p.ctx, p.executable, p.args()...)
	cmd.Stderr = &p.stderrTail
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create Geph stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("create Geph stdout: %w", err)
	}

	startResult := make(chan error, 1)
	go func() {
		startResult <- cmd.Start()
	}()

	select {
	case err = <-startResult:
	case <-startupCtx.Done():
		_ = stdin.Close()
		_ = stdout.Close()
		go reapLateGephStart(cmd, startResult)
		return p.startupContextError(startupCtx, nil)
	}
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("start Geph: %w", err)
	}

	p.cmd, p.stdin = cmd, stdin
	go p.readLoop(stdout)
	go p.writeLoop()

	go func() {
		err := cmd.Wait()
		p.stateMu.Lock()
		p.waitErr = err
		p.closed = true
		p.stateMu.Unlock()
		close(p.waitDone)
		close(p.done)
	}()

	if err = p.waitUntilReady(startupCtx); err != nil {
		_ = p.Close()
		return err
	}
	return nil
}

func reapLateGephStart(cmd *exec.Cmd, startResult <-chan error) {
	if err := <-startResult; err == nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func (p *gephProcess) ensureControlAddressAvailable(ctx context.Context) error {
	if p.controlAddress == "" {
		return nil
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", p.controlAddress)
	if err != nil {
		return fmt.Errorf("Geph control address %s is already in use or unavailable: %w", p.controlAddress, err)
	}
	if err := listener.Close(); err != nil {
		return fmt.Errorf("release Geph control address %s after availability check: %w", p.controlAddress, err)
	}
	return nil
}

func (p *gephProcess) waitUntilReady(startupCtx context.Context) error {
	if p.controlAddress == "" {
		return nil
	}

	ticker := time.NewTicker(controlRPCPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-startupCtx.Done():
			return p.startupContextError(startupCtx, lastErr)
		case <-p.waitDone:
			if p.ctx.Err() != nil {
				return p.startupContextError(startupCtx, lastErr)
			}
			return p.startupExitedError()
		default:
		}

		ready, state, err := p.queryConnectionState(startupCtx)
		if ready {
			select {
			case <-p.waitDone:
				if p.ctx.Err() != nil {
					return p.startupContextError(startupCtx, lastErr)
				}
				return p.startupExitedError()
			default:
				return nil
			}
		}
		if err != nil {
			var protocolErr *gephControlProtocolError
			if errors.As(err, &protocolErr) {
				return fmt.Errorf("invalid Geph control RPC response: %w", err)
			}
			lastErr = err
		} else if state != "" {
			switch state {
			case "Connecting", "Disconnected":
				lastErr = fmt.Errorf("control rpc state: %s", state)
			default:
				return fmt.Errorf("unexpected Geph control state: %s", state)
			}
		}

		select {
		case <-startupCtx.Done():
			return p.startupContextError(startupCtx, lastErr)
		case <-p.waitDone:
			if p.ctx.Err() != nil {
				return p.startupContextError(startupCtx, lastErr)
			}
			return p.startupExitedError()
		case <-ticker.C:
		}
	}
}

func (p *gephProcess) queryConnectionState(ctx context.Context) (bool, string, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, controlRPCTimeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(attemptCtx, "tcp", p.controlAddress)
	if err != nil {
		return false, "", err
	}
	defer conn.Close()

	if deadline, ok := attemptCtx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return false, "", fmt.Errorf("set conn_info deadline: %w", err)
		}
	}

	request := gephConnInfoRequest{
		JSONRPC: "2.0",
		Method:  "conn_info",
		Params:  []string{},
		ID:      gephReadinessRequestID,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return false, "", fmt.Errorf("marshal conn_info request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')
	if err := writeFull(conn, requestBytes); err != nil {
		return false, "", fmt.Errorf("send conn_info request: %w", err)
	}

	reader := bufio.NewReader(io.LimitReader(conn, maxControlRPCResponseSize+1))
	responseLine, err := reader.ReadString('\n')
	if len(responseLine) > maxControlRPCResponseSize {
		return false, "", newGephControlProtocolError("conn_info response exceeds %d bytes", maxControlRPCResponseSize)
	}
	if err != nil {
		return false, "", fmt.Errorf("read conn_info response: %w", err)
	}
	var response gephConnInfoResponse
	if err := json.Unmarshal([]byte(responseLine), &response); err != nil {
		return false, "", newGephControlProtocolError("parse conn_info response: %w", err)
	}
	if response.JSONRPC != "2.0" {
		return false, "", newGephControlProtocolError("conn_info rpc returned JSON-RPC version %q", response.JSONRPC)
	}
	if response.ID != gephReadinessRequestID {
		return false, "", newGephControlProtocolError("conn_info rpc returned mismatched id %q", response.ID)
	}
	if response.Error != nil {
		if response.Error.Message != "" {
			return false, "", newGephControlProtocolError("conn_info rpc error: %s (code=%d)", response.Error.Message, response.Error.Code)
		}
		return false, "", newGephControlProtocolError("conn_info rpc error: code=%d", response.Error.Code)
	}
	if response.Result == nil {
		return false, "", newGephControlProtocolError("conn_info rpc missing result")
	}
	state := strings.TrimSpace(response.Result.State)
	if state == "" {
		return false, "", newGephControlProtocolError("conn_info rpc missing state")
	}
	return state == "Connected", state, nil
}

func writeFull(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		payload = payload[n:]
	}
	return nil
}

func (p *gephProcess) startupContextError(startupCtx context.Context, lastErr error) error {
	if err := p.ctx.Err(); err != nil {
		if lastErr != nil {
			return fmt.Errorf("start Geph: %w (last control RPC error: %v)", err, lastErr)
		}
		return fmt.Errorf("start Geph: %w", err)
	}
	if errors.Is(startupCtx.Err(), context.DeadlineExceeded) {
		return p.startupTimeoutError(lastErr)
	}
	if err := startupCtx.Err(); err != nil {
		return fmt.Errorf("start Geph: %w", err)
	}
	return fmt.Errorf("start Geph: startup canceled")
}

func (p *gephProcess) startupTimeoutError(lastErr error) error {
	reason := strings.TrimSpace(p.stderrTail.String())
	if reason == "" {
		if lastErr != nil {
			return fmt.Errorf("start Geph: timeout waiting for control RPC after %s: %w", p.timeout, lastErr)
		}
		return fmt.Errorf("start Geph: timeout waiting for control RPC after %s", p.timeout)
	}
	if lastErr != nil {
		return fmt.Errorf("start Geph: timeout waiting for control RPC after %s: %w (%s)", p.timeout, lastErr, reason)
	}
	return fmt.Errorf("start Geph: timeout waiting for control RPC after %s (%s)", p.timeout, reason)
}

func (p *gephProcess) startupExitedError() error {
	p.stateMu.Lock()
	waitErr := p.waitErr
	p.stateMu.Unlock()

	reason := strings.TrimSpace(p.stderrTail.String())
	if waitErr == nil {
		if reason == "" {
			return fmt.Errorf("start Geph: process exited during startup")
		}
		return fmt.Errorf("start Geph: process exited during startup: %s", reason)
	}
	if reason == "" {
		return fmt.Errorf("start Geph: process exited during startup: %w", waitErr)
	}
	return fmt.Errorf("start Geph: process exited during startup: %w (%s)", waitErr, reason)
}

func (p *gephProcess) sendPacket(packet []byte) error {
	if len(packet) == 0 || len(packet) > 65535 {
		return fmt.Errorf("invalid Geph packet length: %d", len(packet))
	}
	p.stateMu.Lock()
	closed := p.closed
	p.stateMu.Unlock()
	if closed {
		return io.ErrClosedPipe
	}
	copyPacket := append([]byte(nil), packet...)
	select {
	case <-p.done:
		return io.ErrClosedPipe
	default:
	}
	select {
	case p.outgoing <- copyPacket:
		return nil
	case <-p.done:
		return io.ErrClosedPipe
	}
}

func (p *gephProcess) readLoop(r io.Reader) {
	defer close(p.incoming)
	var length [2]byte
	for {
		if _, err := io.ReadFull(r, length[:]); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(length[:]))
		if n == 0 {
			continue
		}
		packet := make([]byte, n)
		if _, err := io.ReadFull(r, packet); err != nil {
			return
		}
		select {
		case p.incoming <- packet:
		case <-p.done:
			return
		}
	}
}

func (p *gephProcess) writeLoop() {
	for {
		var packet []byte
		select {
		case packet = <-p.outgoing:
		case <-p.done:
			return
		}
		var length [2]byte
		binary.BigEndian.PutUint16(length[:], uint16(len(packet)))
		if _, err := p.stdin.Write(length[:]); err != nil {
			return
		}
		if _, err := p.stdin.Write(packet); err != nil {
			return
		}
	}
}

func (p *gephProcess) Close() error {
	p.closeOnce.Do(func() {
		p.stateMu.Lock()
		p.closed = true
		p.stateMu.Unlock()
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
			<-p.waitDone
		}
	})
	return nil
}

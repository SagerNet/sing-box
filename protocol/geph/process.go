package geph

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type gephProcess struct {
	ctx                context.Context
	executable, config string
	extraArgs          []string
	timeout            time.Duration
	incoming           chan []byte
	outgoing           chan []byte
	done, waitDone     chan struct{}
	closeOnce          sync.Once
	stateMu            sync.Mutex
	closed             bool
	cmd                *exec.Cmd
	stdin              io.WriteCloser
}

func newGephProcess(ctx context.Context, executable, config string, extraArgs []string, timeout time.Duration) *gephProcess {
	return &gephProcess{ctx: ctx, executable: executable, config: config, extraArgs: append([]string(nil), extraArgs...), timeout: timeout, incoming: make(chan []byte, 256), outgoing: make(chan []byte, 256), done: make(chan struct{}), waitDone: make(chan struct{})}
}

func (p *gephProcess) args() []string {
	return append([]string{"--config", p.config, "--stdio-vpn"}, p.extraArgs...)
}

func (p *gephProcess) Start() error {
	if p.timeout <= 0 {
		p.timeout = 15 * time.Second
	}
	cmd := exec.CommandContext(p.ctx, p.executable, p.args()...)
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
	abortStart := make(chan struct{})
	go func() {
		startErr := cmd.Start()
		select {
		case startResult <- startErr:
		case <-abortStart:
			if startErr == nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}()
	select {
	case err = <-startResult:
	case <-time.After(p.timeout):
		close(abortStart)
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("start Geph: timeout after %s", p.timeout)
	}
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("start Geph: %w", err)
	}
	p.cmd, p.stdin = cmd, stdin
	go p.readLoop(stdout)
	go p.writeLoop()
	go func() { _ = cmd.Wait(); close(p.waitDone); close(p.done) }()
	return nil
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

package libbox

import (
	"bytes"
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/daemon"
)

type TailscaleSSHOptions struct {
	EndpointTag  string
	PeerAddress  string
	Username     string
	TerminalType string
	Columns      int32
	Rows         int32
	WidthPixels  int32
	HeightPixels int32
	HostKeys     StringIterator
	ForwardAgent bool
}

type TailscaleSSHHandler interface {
	OnReady()
	OnOutput(data []byte)
	OnAuthBanner(message string)
	OnExit(exitCode int32, signal string, errorMessage string)
	OnError(message string)
}

type tailscaleSSHResize struct {
	columns      int32
	rows         int32
	widthPixels  int32
	heightPixels int32
}

type TailscaleSSHSession struct {
	stream    daemon.StartedService_StartTailscaleSSHSessionClient
	inputCh   chan []byte
	resizeCh  chan tailscaleSSHResize
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeDone chan struct{}
}

func (s *TailscaleSSHSession) SendInput(data []byte) error {
	select {
	case <-s.ctx.Done():
		return os.ErrClosed
	case s.inputCh <- bytes.Clone(data):
		return nil
	}
}

func (s *TailscaleSSHSession) SendResize(columns int32, rows int32, widthPixels int32, heightPixels int32) error {
	resize := tailscaleSSHResize{
		columns:      columns,
		rows:         rows,
		widthPixels:  widthPixels,
		heightPixels: heightPixels,
	}
	select {
	case s.resizeCh <- resize:
		return nil
	case <-s.ctx.Done():
		return os.ErrClosed
	}
}

func (s *TailscaleSSHSession) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		_ = s.stream.CloseSend()
	})
	<-s.closeDone
	return nil
}

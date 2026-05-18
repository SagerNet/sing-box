//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"sync"
)

type UrbTransaction struct {
	peer      *UsbIpPeer
	seqnum    uint32
	direction uint32

	access    sync.Mutex
	canceling bool
	terminal  bool
	response  SubmitResponse
	err       error
	done      chan struct{}
}

func (t *UrbTransaction) SeqNum() uint32 {
	return t.seqnum
}

func (t *UrbTransaction) Direction() uint32 {
	return t.direction
}

func (t *UrbTransaction) Done() <-chan struct{} {
	return t.done
}

func (t *UrbTransaction) Wait() (SubmitResponse, error) {
	<-t.done
	return t.response, t.err
}

func (t *UrbTransaction) Cancel() error {
	t.access.Lock()
	if t.terminal || t.canceling {
		t.access.Unlock()
		return nil
	}
	t.canceling = true
	t.access.Unlock()
	err := t.peer.cancel(t.seqnum)
	if err != nil {
		t.finalize(SubmitResponse{}, ErrCanceled)
	}
	return err
}

func (t *UrbTransaction) finalize(response SubmitResponse, err error) {
	t.access.Lock()
	if t.terminal {
		t.access.Unlock()
		return
	}
	t.terminal = true
	t.response = response
	t.err = err
	t.access.Unlock()
	close(t.done)
}

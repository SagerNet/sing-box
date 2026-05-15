package usbip

import (
	"context"
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

func (t *UrbTransaction) Wait(ctx context.Context) (SubmitResponse, error) {
	select {
	case <-t.done:
	case <-ctx.Done():
		return SubmitResponse{}, ctx.Err()
	}
	t.access.Lock()
	defer t.access.Unlock()
	return t.response, t.err
}

func (t *UrbTransaction) Cancel(ctx context.Context) error {
	t.access.Lock()
	if t.terminal || t.canceling {
		t.access.Unlock()
		return nil
	}
	t.canceling = true
	t.access.Unlock()

	type writeResult struct{ err error }
	resultCh := make(chan writeResult, 1)
	go func() {
		resultCh <- writeResult{err: t.peer.cancel(t.seqnum)}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.finalize(SubmitResponse{}, ErrCanceled)
		}
		return result.err
	case <-ctx.Done():
		return ctx.Err()
	}
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

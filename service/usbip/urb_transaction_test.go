package usbip

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUrbTransactionWaitContextCancel(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	go func() {
		_ = server.readSubmit(t)
		// Never reply.
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 8,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = transaction.Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestUrbTransactionCancelIdempotent(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var unlinks atomic.Int32
	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		// Read exactly one CMD_UNLINK.
		unlink := server.readUnlink(t)
		unlinks.Add(1)
		require.Equal(t, submit.Header.SeqNum, unlink.SeqNum)
		server.writeSubmitResponse(t, USBIPDirIn, submit.Header.SeqNum, usbipStatusECONNRESET, nil, nil)
		server.writeUnlinkResponse(t, unlink.Header.SeqNum, 0)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 8,
	})
	require.NoError(t, err)

	const callers = 8
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			err := transaction.Cancel(context.Background())
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrCanceled)

	serverDone.Wait()
	require.Equal(t, int32(1), unlinks.Load(), "Cancel wrote CMD_UNLINK more than once")
}

func TestUrbTransactionCancelAfterTerminalNoWire(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		server.writeSubmitResponse(t, USBIPDirOut, submit.Header.SeqNum, 0, nil, nil)
		// Any further read MUST be EOF (no CMD_UNLINK in flight).
		_, err := ReadDataHeader(server.conn)
		require.Error(t, err)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirOut,
			Endpoint:  1,
		},
		TransferBufferLength: 4,
		Buffer:               []byte{1, 2, 3, 4},
	})
	require.NoError(t, err)

	_, err = transaction.Wait(context.Background())
	require.NoError(t, err)

	require.NoError(t, transaction.Cancel(context.Background()))

	require.NoError(t, peer.Close())
	serverDone.Wait()
}

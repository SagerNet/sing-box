package v2rayxhttp

import (
	"io"
	"sync"
)

// uploadQueue manages out-of-order packet uploads for packet-up mode
type uploadQueue struct {
	mu           sync.Mutex
	cond         *sync.Cond
	packets      map[int][]byte
	nextSeq      int
	maxBuffered  int
	closed       bool
	currentData  []byte
	currentIndex int
}

func newUploadQueue(maxBuffered int) *uploadQueue {
	q := &uploadQueue{
		packets:     make(map[int][]byte),
		nextSeq:     0,
		maxBuffered: maxBuffered,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *uploadQueue) Push(seq int, data []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	// Store packet
	q.packets[seq] = data

	// Signal readers
	q.cond.Signal()
}

func (q *uploadQueue) Read(b []byte) (n int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		// Return remaining data from current packet
		if q.currentData != nil && q.currentIndex < len(q.currentData) {
			n = copy(b, q.currentData[q.currentIndex:])
			q.currentIndex += n
			if q.currentIndex >= len(q.currentData) {
				q.currentData = nil
				q.currentIndex = 0
			}
			return n, nil
		}

		// Check if closed
		if q.closed && len(q.packets) == 0 {
			return 0, io.EOF
		}

		// Check for next sequential packet
		if data, ok := q.packets[q.nextSeq]; ok {
			delete(q.packets, q.nextSeq)
			q.nextSeq++
			q.currentData = data
			q.currentIndex = 0
			continue
		}

		// Wait for more data
		if q.closed {
			return 0, io.EOF
		}
		q.cond.Wait()
	}
}

func (q *uploadQueue) Write(b []byte) (n int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return 0, io.ErrClosedPipe
	}

	// For stream-up mode, just push data with current sequence
	data := make([]byte, len(b))
	copy(data, b)
	q.packets[q.nextSeq] = data
	q.nextSeq++
	q.cond.Signal()

	return len(b), nil
}

func (q *uploadQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
	return nil
}

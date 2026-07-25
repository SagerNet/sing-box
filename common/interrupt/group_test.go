package interrupt

import (
	"net"
	"sync"
	"testing"
	"time"
)

type closeBarrierConn struct {
	net.Conn
	barrier *sync.WaitGroup
}

func (c *closeBarrierConn) Close() error {
	c.barrier.Done()
	c.barrier.Wait()
	return c.Conn.Close()
}

func TestNestedGroupsInterruptWithoutDeadlock(t *testing.T) {
	groupA := NewGroup()
	groupB := NewGroup()
	barrier := &sync.WaitGroup{}
	barrier.Add(2)

	barrierA, barrierAPeer := net.Pipe()
	barrierB, barrierBPeer := net.Pipe()
	t.Cleanup(func() {
		barrierAPeer.Close()
		barrierBPeer.Close()
	})
	groupA.NewConn(&closeBarrierConn{Conn: barrierA, barrier: barrier}, true)
	groupB.NewConn(&closeBarrierConn{Conn: barrierB, barrier: barrier}, true)

	connA, connAPeer := net.Pipe()
	connB, connBPeer := net.Pipe()
	t.Cleanup(func() {
		connAPeer.Close()
		connBPeer.Close()
	})
	wrapperA := groupA.NewConn(connA, true)
	wrapperB := groupB.NewConn(connB, true)
	groupA.NewConn(wrapperB, true)
	groupB.NewConn(wrapperA, true)

	done := make(chan struct{}, 2)
	go func() {
		groupA.Interrupt(true)
		done <- struct{}{}
	}()
	go func() {
		groupB.Interrupt(true)
		done <- struct{}{}
	}()

	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for range 2 {
		select {
		case <-done:
		case <-timeout.C:
			t.Fatal("nested group interrupt deadlocked")
		}
	}
}

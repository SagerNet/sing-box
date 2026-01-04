package v2rayxhttp

import (
	"io"
	"net"
	"time"
)

// splitConn wraps separate reader and writer into a net.Conn
type splitConn struct {
	reader    io.ReadCloser
	writer    io.WriteCloser
	localAddr net.Addr
	remoteAddr net.Addr
	onClose   func()
}

func newSplitConn(reader io.ReadCloser, writer io.WriteCloser, localAddr, remoteAddr net.Addr, onClose func()) *splitConn {
	return &splitConn{
		reader:    reader,
		writer:    writer,
		localAddr: localAddr,
		remoteAddr: remoteAddr,
		onClose:   onClose,
	}
}

func (c *splitConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *splitConn) Write(b []byte) (n int, err error) {
	return c.writer.Write(b)
}

func (c *splitConn) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	err1 := c.writer.Close()
	err2 := c.reader.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (c *splitConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *splitConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *splitConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *splitConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *splitConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// dummyAddr is a placeholder address for HTTP connections
type dummyAddr struct {
	network string
	address string
}

func (a dummyAddr) Network() string {
	return a.network
}

func (a dummyAddr) String() string {
	return a.address
}

// waitReadCloser wraps an io.ReadCloser with async initialization
type waitReadCloser struct {
	reader io.ReadCloser
	err    error
	done   chan struct{}
}

func newWaitReadCloser() *waitReadCloser {
	return &waitReadCloser{
		done: make(chan struct{}),
	}
}

func (w *waitReadCloser) Set(reader io.ReadCloser, err error) {
	w.reader = reader
	w.err = err
	close(w.done)
}

func (w *waitReadCloser) Read(b []byte) (n int, err error) {
	<-w.done
	if w.err != nil {
		return 0, w.err
	}
	return w.reader.Read(b)
}

func (w *waitReadCloser) Close() error {
	<-w.done
	if w.reader != nil {
		return w.reader.Close()
	}
	return nil
}

// pipeWriter wraps io.PipeWriter as WriteCloser
type pipeWriteCloser struct {
	*io.PipeWriter
}

func (p *pipeWriteCloser) Close() error {
	return p.PipeWriter.Close()
}

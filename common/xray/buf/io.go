package buf

import (
	"io"
	"net"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/common/xray/stat"
	"github.com/sagernet/sing-box/common/xray/stats"
	E "github.com/sagernet/sing/common/exceptions"
)

// Reader extends io.Reader with MultiBuffer.
type Reader interface {
	// ReadMultiBuffer reads content from underlying reader, and put it into a MultiBuffer.
	ReadMultiBuffer() (MultiBuffer, error)
}

// ErrReadTimeout is an error that happens with IO timeout.
var ErrReadTimeout = E.New("IO timeout")

// TimeoutReader is a reader that returns error if Read() operation takes longer than the given timeout.
type TimeoutReader interface {
	ReadMultiBufferTimeout(time.Duration) (MultiBuffer, error)
}

// Writer extends io.Writer with MultiBuffer.
type Writer interface {
	// WriteMultiBuffer writes a MultiBuffer into underlying writer.
	WriteMultiBuffer(MultiBuffer) error
}

// WriteAllBytes ensures all bytes are written into the given writer.
func WriteAllBytes(writer io.Writer, payload []byte, c stats.Counter) error {
	wc := 0
	defer func() {
		if c != nil {
			c.Add(int64(wc))
		}
	}()

	for len(payload) > 0 {
		n, err := writer.Write(payload)
		wc += n
		if err != nil {
			return err
		}
		payload = payload[n:]
	}
	return nil
}

func isPacketReader(reader io.Reader) bool {
	_, ok := reader.(net.PacketConn)
	return ok
}

// NewReader creates a new Reader.
// The Reader instance doesn't take the ownership of reader.
func NewReader(reader io.Reader) Reader {
	if mr, ok := reader.(Reader); ok {
		return mr
	}

	if isPacketReader(reader) {
		return &PacketReader{
			Reader: reader,
		}
	}

	return &SingleReader{
		Reader: reader,
	}
}

// NewPacketReader creates a new PacketReader based on the given reader.
func NewPacketReader(reader io.Reader) Reader {
	if mr, ok := reader.(Reader); ok {
		return mr
	}

	return &PacketReader{
		Reader: reader,
	}
}

func isPacketWriter(writer io.Writer) bool {
	if _, ok := writer.(net.PacketConn); ok {
		return true
	}

	// If the writer doesn't implement syscall.Conn, it is probably not a TCP connection.
	if _, ok := writer.(syscall.Conn); !ok {
		return true
	}
	return false
}

// NewWriter creates a new Writer.
func NewWriter(writer io.Writer) Writer {
	if mw, ok := writer.(Writer); ok {
		return mw
	}

	iConn := writer
	if statConn, ok := writer.(*stat.CounterConnection); ok {
		iConn = statConn.Connection
	}

	if isPacketWriter(iConn) {
		return &SequentialWriter{
			Writer: writer,
		}
	}

	var counter stats.Counter

	if statConn, ok := writer.(*stat.CounterConnection); ok {
		counter = statConn.WriteCounter
	}
	return &BufferToBytesWriter{
		Writer:  iConn,
		counter: counter,
	}
}

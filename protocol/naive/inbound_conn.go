package naive

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
)

const kFirstPaddings = 8

type naiveH1Conn struct {
	net.Conn
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *naiveH1Conn) Read(p []byte) (n int, err error) {
	n, err = c.read(p)
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.Conn, c.paddingRemaining)
		if err != nil {
			return
		}
		c.paddingRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		var paddingHdr []byte
		if len(p) >= 3 {
			paddingHdr = p[:3]
		} else {
			paddingHdr = make([]byte, 3)
		}
		_, err = io.ReadFull(c.Conn, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingSize := int(paddingHdr[2])
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.Conn.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingSize
		return
	}
	return c.Conn.Read(p)
}

func (c *naiveH1Conn) Write(p []byte) (n int, err error) {
	for pLen := len(p); pLen > 0; {
		var data []byte
		if pLen > 65535 {
			data = p[:65535]
			p = p[65535:]
			pLen -= 65535
		} else {
			data = p
			pLen = 0
		}
		var writeN int
		writeN, err = c.write(data)
		n += writeN
		if err != nil {
			break
		}
	}
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingSize := rand.Intn(256)

		buffer := buf.NewSize(3 + len(p) + paddingSize)
		defer buffer.Release()
		header := buffer.Extend(3)
		binary.BigEndian.PutUint16(header, uint16(len(p)))
		header[2] = byte(paddingSize)

		common.Must1(buffer.Write(p))
		_, err = c.Conn.Write(buffer.Bytes())
		if err == nil {
			n = len(p)
		}
		c.writePadding++
		return
	}
	return c.Conn.Write(p)
}

func (c *naiveH1Conn) FrontHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 3
	}
	return 0
}

func (c *naiveH1Conn) RearHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 255
	}
	return 0
}

func (c *naiveH1Conn) WriterMTU() int {
	if c.writePadding < kFirstPaddings {
		return 65535
	}
	return 0
}

func (c *naiveH1Conn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		if bufferLen > 65535 {
			return common.Error(c.Write(buffer.Bytes()))
		}
		paddingSize := rand.Intn(256)
		header := buffer.ExtendHeader(3)
		binary.BigEndian.PutUint16(header, uint16(bufferLen))
		header[2] = byte(paddingSize)
		buffer.Extend(paddingSize)
		c.writePadding++
	}
	return wrapHttpError(common.Error(c.Conn.Write(buffer.Bytes())))
}

// FIXME
/*func (c *naiveH1Conn) WriteTo(w io.Writer) (n int64, err error) {
	if c.readPadding < kFirstPaddings {
		n, err = bufio.WriteToN(c, w, kFirstPaddings-c.readPadding)
	} else {
		n, err = bufio.Copy(w, c.Conn)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH1Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writePadding < kFirstPaddings {
		n, err = bufio.ReadFromN(c, r, kFirstPaddings-c.writePadding)
	} else {
		n, err = bufio.Copy(c.Conn, r)
	}
	return n, wrapHttpError(err)
}
*/

func (c *naiveH1Conn) Upstream() any {
	return c.Conn
}

func (c *naiveH1Conn) ReaderReplaceable() bool {
	return c.readPadding == kFirstPaddings
}

func (c *naiveH1Conn) WriterReplaceable() bool {
	return c.writePadding == kFirstPaddings
}

type naiveH2Conn struct {
	reader           io.Reader
	writer           io.Writer
	flusher          http.Flusher
	rAddr            net.Addr
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func (c *naiveH2Conn) Read(p []byte) (n int, err error) {
	n, err = c.read(p)
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.reader.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.reader, c.paddingRemaining)
		if err != nil {
			return
		}
		c.paddingRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		var paddingHdr []byte
		if len(p) >= 3 {
			paddingHdr = p[:3]
		} else {
			paddingHdr = make([]byte, 3)
		}
		_, err = io.ReadFull(c.reader, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingSize := int(paddingHdr[2])
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.reader.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingSize
		return
	}
	return c.reader.Read(p)
}

func (c *naiveH2Conn) Write(p []byte) (n int, err error) {
	for pLen := len(p); pLen > 0; {
		var data []byte
		if pLen > 65535 {
			data = p[:65535]
			p = p[65535:]
			pLen -= 65535
		} else {
			data = p
			pLen = 0
		}
		var writeN int
		writeN, err = c.write(data)
		n += writeN
		if err != nil {
			break
		}
	}
	if err == nil {
		c.flusher.Flush()
	}
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingSize := rand.Intn(256)

		buffer := buf.NewSize(3 + len(p) + paddingSize)
		defer buffer.Release()
		header := buffer.Extend(3)
		binary.BigEndian.PutUint16(header, uint16(len(p)))
		header[2] = byte(paddingSize)

		common.Must1(buffer.Write(p))
		_, err = c.writer.Write(buffer.Bytes())
		if err == nil {
			n = len(p)
		}
		c.writePadding++
		return
	}
	return c.writer.Write(p)
}

func (c *naiveH2Conn) FrontHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 3
	}
	return 0
}

func (c *naiveH2Conn) RearHeadroom() int {
	if c.writePadding < kFirstPaddings {
		return 255
	}
	return 0
}

func (c *naiveH2Conn) WriterMTU() int {
	if c.writePadding < kFirstPaddings {
		return 65535
	}
	return 0
}

func (c *naiveH2Conn) WriteBuffer(buffer *buf.Buffer) error {
	defer buffer.Release()
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		if bufferLen > 65535 {
			return common.Error(c.Write(buffer.Bytes()))
		}
		paddingSize := rand.Intn(256)
		header := buffer.ExtendHeader(3)
		binary.BigEndian.PutUint16(header, uint16(bufferLen))
		header[2] = byte(paddingSize)
		buffer.Extend(paddingSize)
		c.writePadding++
	}
	err := common.Error(c.writer.Write(buffer.Bytes()))
	if err == nil {
		c.flusher.Flush()
	}
	return wrapHttpError(err)
}

// FIXME
/*func (c *naiveH2Conn) WriteTo(w io.Writer) (n int64, err error) {
	if c.readPadding < kFirstPaddings {
		n, err = bufio.WriteToN(c, w, kFirstPaddings-c.readPadding)
	} else {
		n, err = bufio.Copy(w, c.reader)
	}
	return n, wrapHttpError(err)
}

func (c *naiveH2Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writePadding < kFirstPaddings {
		n, err = bufio.ReadFromN(c, r, kFirstPaddings-c.writePadding)
	} else {
		n, err = bufio.Copy(c.writer, r)
	}
	return n, wrapHttpError(err)
}*/

func (c *naiveH2Conn) Close() error {
	return common.Close(
		c.reader,
		c.writer,
	)
}

func (c *naiveH2Conn) LocalAddr() net.Addr {
	return M.Socksaddr{}
}

func (c *naiveH2Conn) RemoteAddr() net.Addr {
	return c.rAddr
}

func (c *naiveH2Conn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *naiveH2Conn) NeedAdditionalReadDeadline() bool {
	return true
}

func (c *naiveH2Conn) UpstreamReader() any {
	return c.reader
}

func (c *naiveH2Conn) UpstreamWriter() any {
	return c.writer
}

func (c *naiveH2Conn) ReaderReplaceable() bool {
	return c.readPadding == kFirstPaddings
}

func (c *naiveH2Conn) WriterReplaceable() bool {
	return c.writePadding == kFirstPaddings
}

func wrapHttpError(err error) error {
	if err == nil {
		return err
	}
	if strings.Contains(err.Error(), "client disconnected") {
		return net.ErrClosed
	}
	if strings.Contains(err.Error(), "body closed by handler") {
		return net.ErrClosed
	}
	if strings.Contains(err.Error(), "canceled with error code 268") {
		return io.EOF
	}
	return err
}

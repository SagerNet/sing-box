package mux

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

const kFirstPaddings = 16

type paddingConn struct {
	N.ExtendedConn
	writer           N.VectorisedWriter
	readPadding      int
	writePadding     int
	readRemaining    int
	paddingRemaining int
}

func newPaddingConn(conn net.Conn) net.Conn {
	writer, isVectorised := bufio.CreateVectorisedWriter(conn)
	if isVectorised {
		return &vectorisedPaddingConn{
			paddingConn{
				ExtendedConn: bufio.NewExtendedConn(conn),
				writer:       bufio.NewVectorisedWriter(conn),
			},
			writer,
		}
	} else {
		return &paddingConn{
			ExtendedConn: bufio.NewExtendedConn(conn),
			writer:       bufio.NewVectorisedWriter(conn),
		}
	}
}

func (c *paddingConn) Read(p []byte) (n int, err error) {
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.ExtendedConn.Read(p)
		if err != nil {
			return
		}
		c.readRemaining -= n
		return
	}
	if c.paddingRemaining > 0 {
		err = rw.SkipN(c.ExtendedConn, c.paddingRemaining)
		if err != nil {
			return
		}
		c.paddingRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		var paddingHdr []byte
		if len(p) >= 4 {
			paddingHdr = p[:4]
		} else {
			_paddingHdr := make([]byte, 4)
			defer common.KeepAlive(_paddingHdr)
			paddingHdr = common.Dup(_paddingHdr)
		}
		_, err = io.ReadFull(c.ExtendedConn, paddingHdr)
		if err != nil {
			return
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingLen := int(binary.BigEndian.Uint16(paddingHdr[2:]))
		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err = c.ExtendedConn.Read(p)
		if err != nil {
			return
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingLen
		return
	}
	return c.ExtendedConn.Read(p)
}

func (c *paddingConn) Write(p []byte) (n int, err error) {
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
	return n, err
}

func (c *paddingConn) write(p []byte) (n int, err error) {
	if c.writePadding < kFirstPaddings {
		paddingLen := 256 + rand.Intn(512)
		_buffer := buf.StackNewSize(4 + len(p) + paddingLen)
		defer common.KeepAlive(_buffer)
		buffer := common.Dup(_buffer)
		defer buffer.Release()
		header := buffer.Extend(4)
		binary.BigEndian.PutUint16(header[:2], uint16(len(p)))
		binary.BigEndian.PutUint16(header[2:], uint16(paddingLen))
		common.Must1(buffer.Write(p))
		buffer.Extend(paddingLen)
		_, err = c.ExtendedConn.Write(buffer.Bytes())
		if err == nil {
			n = len(p)
		}
		c.writePadding++
		return
	}
	return c.ExtendedConn.Write(p)
}

func (c *paddingConn) ReadBuffer(buffer *buf.Buffer) error {
	p := buffer.FreeBytes()
	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err := c.ExtendedConn.Read(p)
		if err != nil {
			return err
		}
		c.readRemaining -= n
		buffer.Truncate(n)
		return nil
	}
	if c.paddingRemaining > 0 {
		err := rw.SkipN(c.ExtendedConn, c.paddingRemaining)
		if err != nil {
			return err
		}
		c.paddingRemaining = 0
	}
	if c.readPadding < kFirstPaddings {
		var paddingHdr []byte
		if len(p) >= 4 {
			paddingHdr = p[:4]
		} else {
			_paddingHdr := make([]byte, 4)
			defer common.KeepAlive(_paddingHdr)
			paddingHdr = common.Dup(_paddingHdr)
		}
		_, err := io.ReadFull(c.ExtendedConn, paddingHdr)
		if err != nil {
			return err
		}
		originalDataSize := int(binary.BigEndian.Uint16(paddingHdr[:2]))
		paddingLen := int(binary.BigEndian.Uint16(paddingHdr[2:]))

		if len(p) > originalDataSize {
			p = p[:originalDataSize]
		}
		n, err := c.ExtendedConn.Read(p)
		if err != nil {
			return err
		}
		c.readPadding++
		c.readRemaining = originalDataSize - n
		c.paddingRemaining = paddingLen
		buffer.Truncate(n)
		return nil
	}
	return c.ExtendedConn.ReadBuffer(buffer)
}

func (c *paddingConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.writePadding < kFirstPaddings {
		bufferLen := buffer.Len()
		if bufferLen > 65535 {
			return common.Error(c.Write(buffer.Bytes()))
		}
		paddingLen := 256 + rand.Intn(512)
		header := buffer.ExtendHeader(4)
		binary.BigEndian.PutUint16(header[:2], uint16(bufferLen))
		binary.BigEndian.PutUint16(header[2:], uint16(paddingLen))
		buffer.Extend(paddingLen)
		c.writePadding++
	}
	return c.ExtendedConn.WriteBuffer(buffer)
}

func (c *paddingConn) FrontHeadroom() int {
	return 4 + 256 + 1024
}

type vectorisedPaddingConn struct {
	paddingConn
	writer N.VectorisedWriter
}

func (c *vectorisedPaddingConn) WriteVectorised(buffers []*buf.Buffer) error {
	if c.writePadding < kFirstPaddings {
		bufferLen := buf.LenMulti(buffers)
		if bufferLen > 65535 {
			defer buf.ReleaseMulti(buffers)
			for _, buffer := range buffers {
				_, err := c.Write(buffer.Bytes())
				if err != nil {
					return err
				}
			}
			return nil
		}
		paddingLen := 256 + rand.Intn(512)
		header := buf.NewSize(4)
		common.Must(
			binary.Write(header, binary.BigEndian, uint16(bufferLen)),
			binary.Write(header, binary.BigEndian, uint16(paddingLen)),
		)
		c.writePadding++
		padding := buf.NewSize(paddingLen)
		padding.Extend(paddingLen)
		buffers = append(append([]*buf.Buffer{header}, buffers...), padding)
	}
	return c.writer.WriteVectorised(buffers)
}

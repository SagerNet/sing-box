package v2raykcp

import (
	"io"
	"sync"
)

type SegmentWriter interface {
	Write(Segment) error
}

type SimpleSegmentWriter struct {
	sync.Mutex
	buffer []byte
	writer io.Writer
}

func NewSegmentWriter(writer io.Writer) SegmentWriter {
	return &SimpleSegmentWriter{
		buffer: make([]byte, 2048),
		writer: writer,
	}
}

func (w *SimpleSegmentWriter) Write(seg Segment) error {
	w.Lock()
	defer w.Unlock()

	segSize := seg.ByteSize()
	if int(segSize) > len(w.buffer) {
		w.buffer = make([]byte, segSize)
	}
	seg.Serialize(w.buffer[:segSize])
	_, err := w.writer.Write(w.buffer[:segSize])
	return err
}

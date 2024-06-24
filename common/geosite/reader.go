package geosite

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"sync/atomic"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

type Reader struct {
	reader       io.ReadSeeker
	domainIndex  map[string]int
	domainLength map[string]int
}

func Open(path string) (*Reader, []string, error) {
	content, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	reader := &Reader{
		reader: content,
	}
	err = reader.readMetadata()
	if err != nil {
		content.Close()
		return nil, nil, err
	}
	codes := make([]string, 0, len(reader.domainIndex))
	for code := range reader.domainIndex {
		codes = append(codes, code)
	}
	return reader, codes, nil
}

type geositeMetadata struct {
	Code   string
	Index  uint64
	Length uint64
}

func (r *Reader) readMetadata() error {
	reader := bufio.NewReader(r.reader)
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if version != 0 {
		return E.New("unknown version")
	}
	metadataEntries, err := varbin.ReadValue[[]geositeMetadata](reader, binary.BigEndian)
	if err != nil {
		return err
	}
	domainIndex := make(map[string]int)
	domainLength := make(map[string]int)
	for _, entry := range metadataEntries {
		domainIndex[entry.Code] = int(entry.Index)
		domainLength[entry.Code] = int(entry.Length)
	}
	r.domainIndex = domainIndex
	r.domainLength = domainLength
	if reader.Buffered() > 0 {
		return common.Error(r.reader.Seek(int64(-reader.Buffered()), io.SeekCurrent))
	}
	return nil
}

func (r *Reader) Read(code string) ([]Item, error) {
	index, exists := r.domainIndex[code]
	if !exists {
		return nil, E.New("code ", code, " not exists!")
	}
	_, err := r.reader.Seek(int64(index), io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	counter := &readCounter{Reader: r.reader}
	domain, err := varbin.ReadValue[[]Item](bufio.NewReader(counter), binary.BigEndian)
	if err != nil {
		return nil, err
	}
	_, err = r.reader.Seek(int64(-index)-counter.count, io.SeekCurrent)
	return domain, err
}

func (r *Reader) Upstream() any {
	return r.reader
}

type readCounter struct {
	io.Reader
	count int64
}

func (r *readCounter) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if n > 0 {
		atomic.AddInt64(&r.count, int64(n))
	}
	return
}

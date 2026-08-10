package geosite

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"

	E "github.com/sagernet/sing/common/exceptions"
)

type Reader struct {
	access         sync.Mutex
	reader         io.ReadSeeker
	bufferedReader *bufio.Reader
	metadataIndex  int64
	domainIndex    map[string]int
	domainLength   map[string]int
}

func Open(path string) (*Reader, []string, error) {
	content, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	reader, codes, err := NewReader(content)
	if err != nil {
		content.Close()
		return nil, nil, err
	}
	return reader, codes, nil
}

func NewReader(readSeeker io.ReadSeeker) (*Reader, []string, error) {
	reader := &Reader{
		reader: readSeeker,
	}
	err := reader.readMetadata()
	if err != nil {
		return nil, nil, err
	}
	codes := make([]string, 0, len(reader.domainIndex))
	for code := range reader.domainIndex {
		codes = append(codes, code)
	}
	return reader, codes, nil
}

func (r *Reader) readMetadata() error {
	counter := &readCounter{Reader: r.reader}
	reader := bufio.NewReader(counter)
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if version != 0 {
		return E.New("unknown version")
	}
	entryLength, err := binary.ReadUvarint(reader)
	if err != nil {
		return err
	}
	domainIndex := make(map[string]int)
	domainLength := make(map[string]int)
	for range entryLength {
		var (
			code       string
			codeIndex  uint64
			codeLength uint64
		)
		code, err = readString(reader)
		if err != nil {
			return err
		}
		codeIndex, err = binary.ReadUvarint(reader)
		if err != nil {
			return err
		}
		codeLength, err = binary.ReadUvarint(reader)
		if err != nil {
			return err
		}
		if codeIndex > math.MaxInt32 || codeLength > math.MaxInt32 {
			return E.New("invalid metadata entry: ", code)
		}
		domainIndex[code] = int(codeIndex)
		domainLength[code] = int(codeLength)
	}
	r.domainIndex = domainIndex
	r.domainLength = domainLength
	r.metadataIndex = counter.count - int64(reader.Buffered())
	r.bufferedReader = reader
	return nil
}

func (r *Reader) Read(code string) ([]Item, error) {
	r.access.Lock()
	defer r.access.Unlock()

	index, exists := r.domainIndex[code]
	if !exists {
		return nil, E.New("code ", code, " not exists!")
	}
	_, err := r.reader.Seek(r.metadataIndex+int64(index), io.SeekStart)
	if err != nil {
		return nil, err
	}
	r.bufferedReader.Reset(r.reader)
	length := r.domainLength[code]
	var itemList []Item
	for range length {
		var (
			typeByte byte
			value    string
		)
		typeByte, err = r.bufferedReader.ReadByte()
		if err != nil {
			return nil, err
		}
		value, err = readString(r.bufferedReader)
		if err != nil {
			return nil, err
		}
		itemList = append(itemList, Item{Type: ItemType(typeByte), Value: value})
	}
	return itemList, nil
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

func readString(reader io.ByteReader) (string, error) {
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return "", err
	}
	var result []byte
	for range length {
		var value byte
		value, err = reader.ReadByte()
		if err != nil {
			return "", err
		}
		result = append(result, value)
	}
	return string(result), nil
}

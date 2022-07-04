package geosite

import (
	"io"
	"sync"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

type Reader struct {
	reader       io.ReadSeeker
	access       sync.Mutex
	metadataRead bool
	domainIndex  map[string]int
	domainLength map[string]int
}

func (r *Reader) readMetadata() error {
	version, err := rw.ReadByte(r.reader)
	if err != nil {
		return err
	}
	if version != 0 {
		return E.New("unknown version")
	}
	entryLength, err := rw.ReadUVariant(r.reader)
	if err != nil {
		return err
	}
	keys := make([]string, entryLength)
	domainIndex := make(map[string]int)
	domainLength := make(map[string]int)
	for i := 0; i < int(entryLength); i++ {
		var (
			code       string
			codeIndex  uint64
			codeLength uint64
		)
		code, err = rw.ReadVString(r.reader)
		if err != nil {
			return err
		}
		keys[i] = code
		codeIndex, err = rw.ReadUVariant(r.reader)
		if err != nil {
			return err
		}
		codeLength, err = rw.ReadUVariant(r.reader)
		if err != nil {
			return err
		}
		domainIndex[code] = int(codeIndex)
		domainLength[code] = int(codeLength)
	}
	r.domainIndex = domainIndex
	r.domainLength = domainLength
	r.metadataRead = true
	return nil
}

func (r *Reader) Read(code string) ([]Item, error) {
	r.access.Lock()
	defer r.access.Unlock()
	if !r.metadataRead {
		err := r.readMetadata()
		if err != nil {
			return nil, err
		}
	}
	if _, exists := r.domainIndex[code]; !exists {
		return nil, E.New("code ", code, " not exists!")
	}
	counter := &rw.ReadCounter{Reader: r.reader}
	domain := make([]Item, r.domainLength[code])
	for i := range domain {
		var (
			item Item
			err  error
		)
		item.Type, err = rw.ReadByte(counter)
		if err != nil {
			return nil, err
		}
		item.Value, err = rw.ReadVString(counter)
		if err != nil {
			return nil, err
		}
		domain[i] = item
	}
	_, err := r.reader.Seek(int64(r.domainIndex[code])-counter.Count(), io.SeekCurrent)
	return domain, err
}

func (r *Reader) Upstream() any {
	return r.reader
}

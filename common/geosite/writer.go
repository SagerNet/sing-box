package geosite

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/varbin"
)

func Write(writer varbin.Writer, domains map[string][]Item) error {
	keys := make([]string, 0, len(domains))
	for code := range domains {
		keys = append(keys, code)
	}
	sort.Strings(keys)

	content := &bytes.Buffer{}
	index := make(map[string]int)
	for _, code := range keys {
		index[code] = content.Len()
		err := varbin.Write(content, binary.BigEndian, domains[code])
		if err != nil {
			return err
		}
	}

	err := writer.WriteByte(0)
	if err != nil {
		return err
	}

	err = varbin.Write(writer, binary.BigEndian, common.Map(keys, func(it string) *geositeMetadata {
		return &geositeMetadata{
			Code:   it,
			Index:  uint64(index[it]),
			Length: uint64(len(domains[it])),
		}
	}))
	if err != nil {
		return err
	}

	_, err = writer.Write(content.Bytes())
	if err != nil {
		return err
	}

	return nil
}

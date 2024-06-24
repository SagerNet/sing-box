package geosite

import (
	"bytes"
	"encoding/binary"
	"sort"

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
		for _, item := range domains[code] {
			err := varbin.Write(content, binary.BigEndian, item)
			if err != nil {
				return err
			}
		}
	}

	err := writer.WriteByte(0)
	if err != nil {
		return err
	}

	_, err = varbin.WriteUvarint(writer, uint64(len(keys)))
	if err != nil {
		return err
	}

	for _, code := range keys {
		err = varbin.Write(writer, binary.BigEndian, code)
		if err != nil {
			return err
		}
		_, err = varbin.WriteUvarint(writer, uint64(index[code]))
		if err != nil {
			return err
		}
		_, err = varbin.WriteUvarint(writer, uint64(len(domains[code])))
		if err != nil {
			return err
		}
	}

	_, err = writer.Write(content.Bytes())
	if err != nil {
		return err
	}

	return nil
}

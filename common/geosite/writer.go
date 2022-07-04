package geosite

import (
	"bytes"
	"io"
	"sort"

	"github.com/sagernet/sing/common/rw"
)

func Write(writer io.Writer, domains map[string][]Item) error {
	keys := make([]string, 0, len(domains))
	for code := range domains {
		keys = append(keys, code)
	}
	sort.Strings(keys)

	content := &bytes.Buffer{}
	index := make(map[string]int)
	for _, code := range keys {
		index[code] = content.Len()
		for _, domain := range domains[code] {
			err := rw.WriteByte(content, byte(domain.Type))
			if err != nil {
				return err
			}
			if err = rw.WriteVString(content, domain.Value); err != nil {
				return err
			}
		}
	}

	err := rw.WriteByte(writer, 0)
	if err != nil {
		return err
	}

	err = rw.WriteUVariant(writer, uint64(len(keys)))
	if err != nil {
		return err
	}

	for _, code := range keys {
		err = rw.WriteVString(writer, code)
		if err != nil {
			return err
		}
		err = rw.WriteUVariant(writer, uint64(index[code]))
		if err != nil {
			return err
		}
		err = rw.WriteUVariant(writer, uint64(len(domains[code])))
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

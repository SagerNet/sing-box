package geoip

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/oschwald/maxminddb-golang"
)

type Reader struct {
	reader *maxminddb.Reader
}

func Open(path string) (*Reader, []string, error) {
	database, err := maxminddb.Open(path)
	if err != nil {
		return nil, nil, err
	}
	if database.Metadata.DatabaseType != "sing-geoip" {
		database.Close()
		return nil, nil, E.New("incorrect database type, expected sing-geoip, got ", database.Metadata.DatabaseType)
	}
	return &Reader{database}, database.Metadata.Languages, nil
}

func (r *Reader) Lookup(addr netip.Addr) string {
	var code string
	_ = r.reader.Lookup(addr.AsSlice(), &code)
	if code != "" {
		return code
	}
	return "unknown"
}

func (r *Reader) Close() error {
	return r.reader.Close()
}

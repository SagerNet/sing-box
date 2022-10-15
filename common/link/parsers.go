package link

import (
	"fmt"
	"net/url"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

// ParseFunc is parser function to load links, like "vmess://..."
type ParseFunc func(u *url.URL) (Link, error)

// Parser is parser load v2ray links with specified schemes
type Parser struct {
	Name   string
	Scheme []string
	Parse  ParseFunc
}

var (
	parsers = make(map[string][]*Parser)
)

// RegisterParser add a new link parser.
func RegisterParser(parser *Parser) error {
	for _, scheme := range parser.Scheme {
		s := strings.ToLower(scheme)
		ps := parsers[s]
		if len(ps) == 0 {
			ps = make([]*Parser, 0)
		}
		parsers[s] = append(ps, parser)
	}

	return nil
}

func getParsers(u *url.URL) ([]*Parser, error) {
	if u.Scheme == "" {
		return nil, E.New("invalid link")
	}
	s := strings.ToLower(u.Scheme)
	ps := parsers[s]
	if len(ps) == 0 {
		return nil, fmt.Errorf("unsupported link scheme: %s", u.Scheme)
	}
	return ps, nil
}

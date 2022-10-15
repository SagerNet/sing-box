package link

import (
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// Link is the interface for v2ray links
type Link interface {
	// Parse parses the url to Link
	Parse(*url.URL) error
	// Detail returns human readable string
	Options() *option.Outbound
}

// Parse parses a link string to Link
func Parse(u *url.URL) (Link, error) {
	ps, err := getParsers(u)
	if err != nil {
		return nil, err
	}
	errs := make([]error, 0, len(ps))
	for _, p := range ps {
		lk, err := p.Parse(u)
		if err == nil {
			return lk, nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 1 {
		return nil, errs[0]
	}
	return nil, E.Errors(errs...)
}

// ParseCollection parses a links collection string to []Link
func ParseCollection(content string) ([]Link, error) {
	content = doBase64DecodeOrNothing(content)
	links := make([]Link, 0)
	errs := make([]error, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			continue
		}
		link, err := Parse(u)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		links = append(links, link)
	}
	return links, E.Errors(errs...)
}

func doBase64DecodeOrNothing(s string) string {
	b, err := base64Decode(s)
	if err != nil {
		return s
	}
	return string(b)
}

func base64Decode(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	stdb64 := b64
	if pad := len(b64) % 4; pad != 0 {
		stdb64 += strings.Repeat("=", 4-pad)
	}

	b, err := base64.StdEncoding.DecodeString(stdb64)
	if err != nil {
		return base64.URLEncoding.DecodeString(b64)
	}
	return b, nil
}

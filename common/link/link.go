package link

import (
	"net/url"

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

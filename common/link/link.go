package link

import (
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

// Link is the interface for v2ray links
type Link interface {
	// Detail returns human readable string
	Options() *option.Outbound
	// String unmarshals Link to string
	String() string
}

// Parse parses a link string to Link
func Parse(arg string) (Link, error) {
	ps, err := getParsers(arg)
	if err != nil {
		return nil, err
	}
	errs := make([]error, 0, len(ps))
	for _, p := range ps {
		lk, err := p.Parse(arg)
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

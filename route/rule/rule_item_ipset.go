package rule

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/ipset"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
)

var _ RuleItem = (*IPSetItem)(nil)

type IPSetItem struct {
	logger      logger.Logger
	setNames    []string
	description string
}

func NewIPSetItem(logger logger.Logger, setNames []string) (*IPSetItem, error) {
	if len(setNames) == 0 {
		return nil, E.New("no ipset names provided")
	}

	for _, setName := range setNames {
		if err := ipset.Verify(setName); err != nil {
			return nil, err
		}
	}

	// Build description
	description := "ipset="
	if len(setNames) == 1 {
		description += setNames[0]
	} else if len(setNames) <= 3 {
		description += "[" + strings.Join(setNames, " ") + "]"
	} else {
		description += "[" + strings.Join(setNames[:3], " ") + "...]"
	}

	return &IPSetItem{
		logger:      logger,
		setNames:    setNames,
		description: description,
	}, nil
}

func (r *IPSetItem) Match(metadata *adapter.InboundContext) bool {
	var addr netip.Addr
	if metadata.Destination.IsIP() {
		addr = metadata.Destination.Addr
	} else if len(metadata.DestinationAddresses) > 0 {
		addr = metadata.DestinationAddresses[0]
	} else {
		return false
	}

	if !addr.IsValid() {
		return false
	}

	ip := net.IP(addr.AsSlice())

	// Check against all configured sets
	for _, setName := range r.setNames {
		exist, err := ipset.Test(setName, ip)
		if err != nil {
			r.logger.Warn(E.Cause(err, fmt.Sprintf("check ipset '%s' failed", setName)))
			return false
		}
		if exist {
			return true
		}
	}
	return false
}

func (r *IPSetItem) String() string {
	return r.description
}

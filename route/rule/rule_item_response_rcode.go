package rule

import (
	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"

	"github.com/miekg/dns"
)

var _ RuleItem = (*DNSResponseRCodeItem)(nil)

type DNSResponseRCodeItem struct {
	rcode int
}

func NewDNSResponseRCodeItem(rcode int) *DNSResponseRCodeItem {
	return &DNSResponseRCodeItem{rcode: rcode}
}

func (r *DNSResponseRCodeItem) Match(metadata *adapter.InboundContext) bool {
	return metadata.DNSResponse != nil && metadata.DNSResponse.Rcode == r.rcode
}

func (r *DNSResponseRCodeItem) String() string {
	return F.ToString("response_rcode=", dns.RcodeToString[r.rcode])
}

package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"

	"github.com/miekg/dns"
)

var _ RuleItem = (*DNSResponseRecordItem)(nil)

type DNSResponseRecordItem struct {
	field    string
	records  []option.DNSRecordOptions
	selector func(*dns.Msg) []dns.RR
}

func NewDNSResponseRecordItem(field string, records []option.DNSRecordOptions, selector func(*dns.Msg) []dns.RR) *DNSResponseRecordItem {
	return &DNSResponseRecordItem{
		field:    field,
		records:  records,
		selector: selector,
	}
}

func (r *DNSResponseRecordItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.DNSResponse == nil {
		return false
	}
	records := r.selector(metadata.DNSResponse)
	for _, expected := range r.records {
		for _, record := range records {
			if expected.Match(record) {
				return true
			}
		}
	}
	return false
}

func (r *DNSResponseRecordItem) String() string {
	descriptions := make([]string, 0, len(r.records))
	for _, record := range r.records {
		if record.RR != nil {
			descriptions = append(descriptions, record.RR.String())
		}
	}
	return r.field + "=[" + strings.Join(descriptions, " ") + "]"
}

func dnsResponseAnswers(message *dns.Msg) []dns.RR {
	return message.Answer
}

func dnsResponseNS(message *dns.Msg) []dns.RR {
	return message.Ns
}

func dnsResponseExtra(message *dns.Msg) []dns.RR {
	return message.Extra
}

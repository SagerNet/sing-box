package option

import (
	"encoding/base64"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/miekg/dns"
)

type PredefinedDNSServerOptions struct {
	Responses []DNSResponseOptions `json:"responses,omitempty"`
}

type DNSResponseOptions struct {
	Query     badoption.Listable[string]       `json:"query,omitempty"`
	QueryType badoption.Listable[DNSQueryType] `json:"query_type,omitempty"`

	RCode  *DNSRCode                            `json:"rcode,omitempty"`
	Answer badoption.Listable[DNSRecordOptions] `json:"answer,omitempty"`
	Ns     badoption.Listable[DNSRecordOptions] `json:"ns,omitempty"`
	Extra  badoption.Listable[DNSRecordOptions] `json:"extra,omitempty"`
}

type DNSRCode int

func (r DNSRCode) MarshalJSON() ([]byte, error) {
	rCodeValue, loaded := dns.RcodeToString[int(r)]
	if loaded {
		return json.Marshal(rCodeValue)
	}
	return json.Marshal(int(r))
}

func (r *DNSRCode) UnmarshalJSON(bytes []byte) error {
	var intValue int
	err := json.Unmarshal(bytes, &intValue)
	if err == nil {
		*r = DNSRCode(intValue)
		return nil
	}
	var stringValue string
	err = json.Unmarshal(bytes, &stringValue)
	if err != nil {
		return err
	}
	rCodeValue, loaded := dns.StringToRcode[stringValue]
	if !loaded {
		return E.New("unknown rcode: " + stringValue)
	}
	*r = DNSRCode(rCodeValue)
	return nil
}

func (r *DNSRCode) Build() int {
	if r == nil {
		return dns.RcodeSuccess
	}
	return int(*r)
}

func (o DNSResponseOptions) Build() ([]dns.Question, *dns.Msg, error) {
	var questions []dns.Question
	if len(o.Query) == 0 && len(o.QueryType) == 0 {
		questions = []dns.Question{{Qclass: dns.ClassINET}}
	} else if len(o.Query) == 0 {
		for _, queryType := range o.QueryType {
			questions = append(questions, dns.Question{
				Qtype:  uint16(queryType),
				Qclass: dns.ClassINET,
			})
		}
	} else if len(o.QueryType) == 0 {
		for _, domain := range o.Query {
			questions = append(questions, dns.Question{
				Name:   dns.Fqdn(domain),
				Qclass: dns.ClassINET,
			})
		}
	} else {
		for _, queryType := range o.QueryType {
			for _, domain := range o.Query {
				questions = append(questions, dns.Question{
					Name:   dns.Fqdn(domain),
					Qtype:  uint16(queryType),
					Qclass: dns.ClassINET,
				})
			}
		}
	}
	return questions, &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Response:           true,
			Rcode:              o.RCode.Build(),
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
		},
		Answer: common.Map(o.Answer, DNSRecordOptions.build),
		Ns:     common.Map(o.Ns, DNSRecordOptions.build),
		Extra:  common.Map(o.Extra, DNSRecordOptions.build),
	}, nil
}

type DNSRecordOptions struct {
	dns.RR
	fromBase64 bool
}

func (o DNSRecordOptions) MarshalJSON() ([]byte, error) {
	if o.fromBase64 {
		buffer := buf.Get(dns.Len(o.RR))
		defer buf.Put(buffer)
		offset, err := dns.PackRR(o.RR, buffer, 0, nil, false)
		if err != nil {
			return nil, err
		}
		return json.Marshal(base64.StdEncoding.EncodeToString(buffer[:offset]))
	}
	return json.Marshal(o.RR.String())
}

func (o *DNSRecordOptions) UnmarshalJSON(data []byte) error {
	var stringValue string
	err := json.Unmarshal(data, &stringValue)
	if err != nil {
		return err
	}
	binary, err := base64.StdEncoding.DecodeString(stringValue)
	if err == nil {
		return o.unmarshalBase64(binary)
	}
	record, err := dns.NewRR(stringValue)
	if err != nil {
		return err
	}
	if a, isA := record.(*dns.A); isA {
		a.A = M.AddrFromIP(a.A).Unmap().AsSlice()
	}
	o.RR = record
	return nil
}

func (o *DNSRecordOptions) unmarshalBase64(binary []byte) error {
	record, _, err := dns.UnpackRR(binary, 0)
	if err != nil {
		return E.New("parse binary DNS record")
	}
	o.RR = record
	o.fromBase64 = true
	return nil
}

func (o DNSRecordOptions) build() dns.RR {
	return o.RR
}

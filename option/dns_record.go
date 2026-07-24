package option

import (
	"cmp"
	"encoding/base64"
	"slices"
	"strings"

	"github.com/sagernet/sing-box/schema"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/miekg/dns"
)

const defaultDNSRecordTTL uint32 = 3600

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

func (r DNSRCode) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return builder.Define("DNSRCode", func() (*schema.Node, error) {
		type rCodeName struct {
			name      string
			value     int
			canonical bool
		}
		rCodeNames := make([]rCodeName, 0, len(dns.StringToRcode))
		for name, value := range dns.StringToRcode {
			canonicalName, canonical := dns.RcodeToString[value]
			rCodeNames = append(rCodeNames, rCodeName{
				name:      name,
				value:     value,
				canonical: canonical && canonicalName == name,
			})
		}
		slices.SortFunc(rCodeNames, func(left rCodeName, right rCodeName) int {
			comparison := cmp.Compare(left.value, right.value)
			if comparison != 0 {
				return comparison
			}
			if left.canonical != right.canonical {
				if left.canonical {
					return -1
				}
				return 1
			}
			return cmp.Compare(left.name, right.name)
		})
		values := make([]string, 0, len(rCodeNames))
		for _, entry := range rCodeNames {
			values = append(values, entry.name)
		}
		return schema.AnyOf(schema.IntegerNode(), schema.StringEnum(values...)), nil
	})
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
	record, err := parseDNSRecord(stringValue)
	if err != nil {
		return err
	}
	if record == nil {
		return E.New("empty DNS record")
	}
	if a, isA := record.(*dns.A); isA {
		a.A = M.AddrFromIP(a.A).Unmap().AsSlice()
	}
	o.RR = record
	return nil
}

func parseDNSRecord(stringValue string) (dns.RR, error) {
	if len(stringValue) > 0 && stringValue[len(stringValue)-1] != '\n' {
		stringValue += "\n"
	}
	parser := dns.NewZoneParser(strings.NewReader(stringValue), "", "")
	parser.SetDefaultTTL(defaultDNSRecordTTL)
	record, _ := parser.Next()
	return record, parser.Err()
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

func (o DNSRecordOptions) Build() dns.RR {
	return o.RR
}

func (o DNSRecordOptions) Match(record dns.RR) bool {
	if o.RR == nil || record == nil {
		return false
	}
	return dns.IsDuplicate(o.RR, record)
}

func (o DNSRecordOptions) DescribeSchema(builder schema.Builder) (*schema.Node, error) {
	return schema.StringNode(), nil
}

package option

import (
	"encoding/base64"

	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/miekg/dns"
)

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

func (o DNSRecordOptions) Build() dns.RR {
	return o.RR
}

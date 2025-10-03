package dns

import (
	"github.com/sagernet/sing/common/buf"

	"github.com/miekg/dns"
)

func TruncateDNSMessage(request *dns.Msg, response *dns.Msg, headroom int) (*buf.Buffer, error) {
	maxLen := 512
	if edns0Option := request.IsEdns0(); edns0Option != nil {
		if udpSize := int(edns0Option.UDPSize()); udpSize > 512 {
			maxLen = udpSize
		}
	}
	responseLen := response.Len()
	if responseLen > maxLen {
		response = response.Copy()
		response.Truncate(maxLen)
	}
	buffer := buf.NewSize(headroom*2 + 1 + responseLen)
	buffer.Resize(headroom, 0)
	rawMessage, err := response.PackBuffer(buffer.FreeBytes())
	if err != nil {
		buffer.Release()
		return nil, err
	}
	buffer.Truncate(len(rawMessage))
	return buffer, nil
}

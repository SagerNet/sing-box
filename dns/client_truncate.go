package dns

import (
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"

	"github.com/miekg/dns"
)

func TruncateDNSMessage(request *dns.Msg, response *dns.Msg, frontHeadroom int, rearHeadroom int) (*buf.Buffer, error) {
	maxLen := 512
	if edns0Option := request.IsEdns0(); edns0Option != nil {
		if udpSize := int(edns0Option.UDPSize()); udpSize > 512 {
			maxLen = udpSize
		}
	}
	responseLen := response.Len()
	compressedResponse := response.Copy()
	compressedResponse.Compress = true
	if compressedResponse.Len() > maxLen {
		compressedResponse.Truncate(maxLen)
		compressedResponse.Compress = true
	}
	buffer := buf.NewSize(frontHeadroom + responseLen + 1 + rearHeadroom)
	buffer.Resize(frontHeadroom, 0)
	rawMessage, err := compressedResponse.PackBuffer(buffer.FreeBytes())
	if err != nil {
		buffer.Release()
		return nil, err
	}
	common.Must1(buffer.Write(rawMessage))
	return buffer, nil
}

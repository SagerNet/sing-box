package dns

import (
	"net/netip"

	"github.com/miekg/dns"
)

func SetClientSubnet(message *dns.Msg, clientSubnet netip.Prefix) *dns.Msg {
	return setClientSubnet(message, clientSubnet, true)
}

func clientSubnetFromMessage(message *dns.Msg) netip.Prefix {
	for _, record := range message.Extra {
		optRecord, isOPTRecord := record.(*dns.OPT)
		if !isOPTRecord {
			continue
		}
		for _, option := range optRecord.Option {
			subnetOption, isEDNS0Subnet := option.(*dns.EDNS0_SUBNET)
			if !isEDNS0Subnet {
				continue
			}
			address, addressLoaded := netip.AddrFromSlice(subnetOption.Address)
			if !addressLoaded {
				return netip.Prefix{}
			}
			return netip.PrefixFrom(address, int(subnetOption.SourceNetmask))
		}
	}
	return netip.Prefix{}
}

func setClientSubnet(message *dns.Msg, clientSubnet netip.Prefix, clone bool) *dns.Msg {
	var (
		optRecord    *dns.OPT
		subnetOption *dns.EDNS0_SUBNET
	)
findExists:
	for _, record := range message.Extra {
		var isOPTRecord bool
		if optRecord, isOPTRecord = record.(*dns.OPT); isOPTRecord {
			for _, option := range optRecord.Option {
				var isEDNS0Subnet bool
				subnetOption, isEDNS0Subnet = option.(*dns.EDNS0_SUBNET)
				if isEDNS0Subnet {
					break findExists
				}
			}
		}
	}
	if optRecord == nil {
		exMessage := *message
		message = &exMessage
		optRecord = &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
		}
		message.Extra = append(message.Extra, optRecord)
	} else if clone {
		return setClientSubnet(message.Copy(), clientSubnet, false)
	}
	if subnetOption == nil {
		subnetOption = new(dns.EDNS0_SUBNET)
		subnetOption.Code = dns.EDNS0SUBNET
		optRecord.Option = append(optRecord.Option, subnetOption)
	}
	if clientSubnet.Addr().Is4() {
		subnetOption.Family = 1
	} else {
		subnetOption.Family = 2
	}
	subnetOption.SourceNetmask = uint8(clientSubnet.Bits())
	subnetOption.Address = clientSubnet.Addr().AsSlice()
	return message
}

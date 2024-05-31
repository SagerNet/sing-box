//go:build linux

package inbound

import (
	"net"
	"net/netip"

	"github.com/sagernet/nftables"
	"github.com/sagernet/nftables/binaryutil"
	"github.com/sagernet/nftables/expr"
	"github.com/sagernet/sing/common/ranges"

	"golang.org/x/sys/unix"
)

func nftablesIfname(n string) []byte {
	b := make([]byte, 16)
	copy(b, n+"\x00")
	return b
}

func nftablesRuleIfName(key expr.MetaKey, value string, exprs ...expr.Any) []expr.Any {
	newExprs := []expr.Any{
		&expr.Meta{Key: key, Register: 1},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     nftablesIfname(value),
		},
	}
	newExprs = append(newExprs, exprs...)
	return newExprs
}

func nftablesRuleMetaUInt32Range(key expr.MetaKey, uidRange ranges.Range[uint32], exprs ...expr.Any) []expr.Any {
	newExprs := []expr.Any{
		&expr.Meta{Key: key, Register: 1},
		&expr.Range{
			Op:       expr.CmpOpEq,
			Register: 1,
			FromData: binaryutil.BigEndian.PutUint32(uidRange.Start),
			ToData:   binaryutil.BigEndian.PutUint32(uidRange.End),
		},
	}
	newExprs = append(newExprs, exprs...)
	return newExprs
}

func nftablesRuleDestinationAddress(address netip.Prefix, exprs ...expr.Any) []expr.Any {
	var newExprs []expr.Any
	if address.Addr().Is4() {
		newExprs = append(newExprs, &expr.Payload{
			OperationType:  expr.PayloadLoad,
			DestRegister:   1,
			SourceRegister: 0,
			Base:           expr.PayloadBaseNetworkHeader,
			Offset:         16,
			Len:            4,
		}, &expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            4,
			Xor:            make([]byte, 4),
			Mask:           net.CIDRMask(address.Bits(), 32),
		})
	} else {
		newExprs = append(newExprs, &expr.Payload{
			OperationType:  expr.PayloadLoad,
			DestRegister:   1,
			SourceRegister: 0,
			Base:           expr.PayloadBaseNetworkHeader,
			Offset:         24,
			Len:            16,
		}, &expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            16,
			Xor:            make([]byte, 16),
			Mask:           net.CIDRMask(address.Bits(), 128),
		})
	}
	newExprs = append(newExprs, &expr.Cmp{
		Op:       expr.CmpOpEq,
		Register: 1,
		Data:     address.Masked().Addr().AsSlice(),
	})
	newExprs = append(newExprs, exprs...)
	return newExprs
}

func nftablesRuleHijackDNS(family nftables.TableFamily, dnsServerAddress netip.Addr) []expr.Any {
	return []expr.Any{
		&expr.Meta{
			Key:      expr.MetaKeyL4PROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.IPPROTO_UDP},
		},
		&expr.Payload{
			OperationType:  expr.PayloadLoad,
			DestRegister:   1,
			SourceRegister: 0,
			Base:           expr.PayloadBaseTransportHeader,
			Offset:         2,
			Len:            2,
		}, &expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     binaryutil.BigEndian.PutUint16(53),
		}, &expr.Immediate{
			Register: 1,
			Data:     dnsServerAddress.AsSlice(),
		}, &expr.NAT{
			Type:       expr.NATTypeDestNAT,
			Family:     uint32(family),
			RegAddrMin: 1,
		},
	}
}

const (
	NF_NAT_RANGE_MAP_IPS = 1 << iota
	NF_NAT_RANGE_PROTO_SPECIFIED
	NF_NAT_RANGE_PROTO_RANDOM
	NF_NAT_RANGE_PERSISTENT
	NF_NAT_RANGE_PROTO_RANDOM_FULLY
	NF_NAT_RANGE_PROTO_OFFSET
)

func nftablesRuleRedirectToPorts(redirectPort uint16) []expr.Any {
	return []expr.Any{
		&expr.Meta{
			Key:      expr.MetaKeyL4PROTO,
			Register: 1,
		},
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{unix.IPPROTO_TCP},
		},
		&expr.Immediate{
			Register: 1,
			Data:     binaryutil.BigEndian.PutUint16(redirectPort),
		}, &expr.Redir{
			RegisterProtoMin: 1,
			Flags:            NF_NAT_RANGE_PROTO_SPECIFIED,
		},
	}
}

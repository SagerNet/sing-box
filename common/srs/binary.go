package srs

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"io"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/domain"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/varbin"

	"go4.org/netipx"
)

var MagicBytes = [3]byte{0x53, 0x52, 0x53} // SRS

const (
	ruleItemQueryType uint8 = iota
	ruleItemNetwork
	ruleItemDomain
	ruleItemDomainKeyword
	ruleItemDomainRegex
	ruleItemSourceIPCIDR
	ruleItemIPCIDR
	ruleItemSourcePort
	ruleItemSourcePortRange
	ruleItemPort
	ruleItemPortRange
	ruleItemProcessName
	ruleItemProcessPath
	ruleItemPackageName
	ruleItemWIFISSID
	ruleItemWIFIBSSID
	ruleItemAdGuardDomain
	ruleItemProcessPathRegex
	ruleItemNetworkType
	ruleItemNetworkIsExpensive
	ruleItemNetworkIsConstrained
	ruleItemNetworkInterfaceAddress
	ruleItemDefaultInterfaceAddress
	ruleItemFinal uint8 = 0xFF
)

func Read(reader io.Reader, recover bool) (ruleSetCompat option.PlainRuleSetCompat, err error) {
	var magicBytes [3]byte
	_, err = io.ReadFull(reader, magicBytes[:])
	if err != nil {
		return
	}
	if magicBytes != MagicBytes {
		err = E.New("invalid sing-box rule-set file")
		return
	}
	var version uint8
	err = binary.Read(reader, binary.BigEndian, &version)
	if err != nil {
		return ruleSetCompat, err
	}
	if version > C.RuleSetVersionCurrent {
		return ruleSetCompat, E.New("unsupported version: ", version)
	}
	compressReader, err := zlib.NewReader(reader)
	if err != nil {
		return
	}
	bReader := bufio.NewReader(compressReader)
	length, err := binary.ReadUvarint(bReader)
	if err != nil {
		return
	}
	ruleSetCompat.Version = version
	ruleSetCompat.Options.Rules = make([]option.HeadlessRule, length)
	for i := uint64(0); i < length; i++ {
		ruleSetCompat.Options.Rules[i], err = readRule(bReader, recover)
		if err != nil {
			err = E.Cause(err, "read rule[", i, "]")
			return
		}
	}
	return
}

func Write(writer io.Writer, ruleSet option.PlainRuleSet, generateVersion uint8) error {
	_, err := writer.Write(MagicBytes[:])
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, generateVersion)
	if err != nil {
		return err
	}
	compressWriter, err := zlib.NewWriterLevel(writer, zlib.BestCompression)
	if err != nil {
		return err
	}
	bWriter := bufio.NewWriter(compressWriter)
	_, err = varbin.WriteUvarint(bWriter, uint64(len(ruleSet.Rules)))
	if err != nil {
		return err
	}
	for _, rule := range ruleSet.Rules {
		err = writeRule(bWriter, rule, generateVersion)
		if err != nil {
			return err
		}
	}
	err = bWriter.Flush()
	if err != nil {
		return err
	}
	return compressWriter.Close()
}

func readRule(reader varbin.Reader, recover bool) (rule option.HeadlessRule, err error) {
	var ruleType uint8
	err = binary.Read(reader, binary.BigEndian, &ruleType)
	if err != nil {
		return
	}
	switch ruleType {
	case 0:
		rule.Type = C.RuleTypeDefault
		rule.DefaultOptions, err = readDefaultRule(reader, recover)
	case 1:
		rule.Type = C.RuleTypeLogical
		rule.LogicalOptions, err = readLogicalRule(reader, recover)
	default:
		err = E.New("unknown rule type: ", ruleType)
	}
	return
}

func writeRule(writer varbin.Writer, rule option.HeadlessRule, generateVersion uint8) error {
	switch rule.Type {
	case C.RuleTypeDefault:
		return writeDefaultRule(writer, rule.DefaultOptions, generateVersion)
	case C.RuleTypeLogical:
		return writeLogicalRule(writer, rule.LogicalOptions, generateVersion)
	default:
		panic("unknown rule type: " + rule.Type)
	}
}

func readDefaultRule(reader varbin.Reader, recover bool) (rule option.DefaultHeadlessRule, err error) {
	var lastItemType uint8
	for {
		var itemType uint8
		err = binary.Read(reader, binary.BigEndian, &itemType)
		if err != nil {
			return
		}
		switch itemType {
		case ruleItemQueryType:
			var rawQueryType []uint16
			rawQueryType, err = readRuleItemUint16(reader)
			if err != nil {
				return
			}
			rule.QueryType = common.Map(rawQueryType, func(it uint16) option.DNSQueryType {
				return option.DNSQueryType(it)
			})
		case ruleItemNetwork:
			rule.Network, err = readRuleItemString(reader)
		case ruleItemDomain:
			var matcher *domain.Matcher
			matcher, err = domain.ReadMatcher(reader)
			if err != nil {
				return
			}
			rule.DomainMatcher = matcher
			if recover {
				rule.Domain, rule.DomainSuffix = matcher.Dump()
			}
		case ruleItemDomainKeyword:
			rule.DomainKeyword, err = readRuleItemString(reader)
		case ruleItemDomainRegex:
			rule.DomainRegex, err = readRuleItemString(reader)
		case ruleItemSourceIPCIDR:
			rule.SourceIPSet, err = readIPSet(reader)
			if err != nil {
				return
			}
			if recover {
				rule.SourceIPCIDR = common.Map(rule.SourceIPSet.Prefixes(), netip.Prefix.String)
			}
		case ruleItemIPCIDR:
			rule.IPSet, err = readIPSet(reader)
			if err != nil {
				return
			}
			if recover {
				rule.IPCIDR = common.Map(rule.IPSet.Prefixes(), netip.Prefix.String)
			}
		case ruleItemSourcePort:
			rule.SourcePort, err = readRuleItemUint16(reader)
		case ruleItemSourcePortRange:
			rule.SourcePortRange, err = readRuleItemString(reader)
		case ruleItemPort:
			rule.Port, err = readRuleItemUint16(reader)
		case ruleItemPortRange:
			rule.PortRange, err = readRuleItemString(reader)
		case ruleItemProcessName:
			rule.ProcessName, err = readRuleItemString(reader)
		case ruleItemProcessPath:
			rule.ProcessPath, err = readRuleItemString(reader)
		case ruleItemProcessPathRegex:
			rule.ProcessPathRegex, err = readRuleItemString(reader)
		case ruleItemPackageName:
			rule.PackageName, err = readRuleItemString(reader)
		case ruleItemWIFISSID:
			rule.WIFISSID, err = readRuleItemString(reader)
		case ruleItemWIFIBSSID:
			rule.WIFIBSSID, err = readRuleItemString(reader)
		case ruleItemAdGuardDomain:
			var matcher *domain.AdGuardMatcher
			matcher, err = domain.ReadAdGuardMatcher(reader)
			if err != nil {
				return
			}
			rule.AdGuardDomainMatcher = matcher
			if recover {
				rule.AdGuardDomain = matcher.Dump()
			}
		case ruleItemNetworkType:
			rule.NetworkType, err = readRuleItemUint8[option.InterfaceType](reader)
		case ruleItemNetworkIsExpensive:
			rule.NetworkIsExpensive = true
		case ruleItemNetworkIsConstrained:
			rule.NetworkIsConstrained = true
		case ruleItemNetworkInterfaceAddress:
			rule.NetworkInterfaceAddress = new(badjson.TypedMap[option.InterfaceType, badoption.Listable[*badoption.Prefixable]])
			var size uint64
			size, err = binary.ReadUvarint(reader)
			if err != nil {
				return
			}
			for i := uint64(0); i < size; i++ {
				var key uint8
				err = binary.Read(reader, binary.BigEndian, &key)
				if err != nil {
					return
				}
				var value []*badoption.Prefixable
				var prefixCount uint64
				prefixCount, err = binary.ReadUvarint(reader)
				if err != nil {
					return
				}
				for j := uint64(0); j < prefixCount; j++ {
					var prefix netip.Prefix
					prefix, err = readPrefix(reader)
					if err != nil {
						return
					}
					value = append(value, common.Ptr(badoption.Prefixable(prefix)))
				}
				rule.NetworkInterfaceAddress.Put(option.InterfaceType(key), value)
			}
		case ruleItemDefaultInterfaceAddress:
			var value []*badoption.Prefixable
			var prefixCount uint64
			prefixCount, err = binary.ReadUvarint(reader)
			if err != nil {
				return
			}
			for j := uint64(0); j < prefixCount; j++ {
				var prefix netip.Prefix
				prefix, err = readPrefix(reader)
				if err != nil {
					return
				}
				value = append(value, common.Ptr(badoption.Prefixable(prefix)))
			}
			rule.DefaultInterfaceAddress = value
		case ruleItemFinal:
			err = binary.Read(reader, binary.BigEndian, &rule.Invert)
			return
		default:
			err = E.New("unknown rule item type: ", itemType, ", last type: ", lastItemType)
		}
		if err != nil {
			return
		}
		lastItemType = itemType
	}
}

func writeDefaultRule(writer varbin.Writer, rule option.DefaultHeadlessRule, generateVersion uint8) error {
	err := binary.Write(writer, binary.BigEndian, uint8(0))
	if err != nil {
		return err
	}
	if len(rule.QueryType) > 0 {
		err = writeRuleItemUint16(writer, ruleItemQueryType, common.Map(rule.QueryType, func(it option.DNSQueryType) uint16 {
			return uint16(it)
		}))
		if err != nil {
			return err
		}
	}
	if len(rule.Network) > 0 {
		err = writeRuleItemString(writer, ruleItemNetwork, rule.Network)
		if err != nil {
			return err
		}
	}
	if len(rule.Domain) > 0 || len(rule.DomainSuffix) > 0 {
		err = binary.Write(writer, binary.BigEndian, ruleItemDomain)
		if err != nil {
			return err
		}
		err = domain.NewMatcher(rule.Domain, rule.DomainSuffix, generateVersion == C.RuleSetVersion1).Write(writer)
		if err != nil {
			return err
		}
	}
	if len(rule.DomainKeyword) > 0 {
		err = writeRuleItemString(writer, ruleItemDomainKeyword, rule.DomainKeyword)
		if err != nil {
			return err
		}
	}
	if len(rule.DomainRegex) > 0 {
		err = writeRuleItemString(writer, ruleItemDomainRegex, rule.DomainRegex)
		if err != nil {
			return err
		}
	}
	if len(rule.SourceIPCIDR) > 0 {
		err = writeRuleItemCIDR(writer, ruleItemSourceIPCIDR, rule.SourceIPCIDR)
		if err != nil {
			return E.Cause(err, "source_ip_cidr")
		}
	}
	if len(rule.IPCIDR) > 0 {
		err = writeRuleItemCIDR(writer, ruleItemIPCIDR, rule.IPCIDR)
		if err != nil {
			return E.Cause(err, "ipcidr")
		}
	}
	if len(rule.SourcePort) > 0 {
		err = writeRuleItemUint16(writer, ruleItemSourcePort, rule.SourcePort)
		if err != nil {
			return err
		}
	}
	if len(rule.SourcePortRange) > 0 {
		err = writeRuleItemString(writer, ruleItemSourcePortRange, rule.SourcePortRange)
		if err != nil {
			return err
		}
	}
	if len(rule.Port) > 0 {
		err = writeRuleItemUint16(writer, ruleItemPort, rule.Port)
		if err != nil {
			return err
		}
	}
	if len(rule.PortRange) > 0 {
		err = writeRuleItemString(writer, ruleItemPortRange, rule.PortRange)
		if err != nil {
			return err
		}
	}
	if len(rule.ProcessName) > 0 {
		err = writeRuleItemString(writer, ruleItemProcessName, rule.ProcessName)
		if err != nil {
			return err
		}
	}
	if len(rule.ProcessPath) > 0 {
		err = writeRuleItemString(writer, ruleItemProcessPath, rule.ProcessPath)
		if err != nil {
			return err
		}
	}
	if len(rule.ProcessPathRegex) > 0 {
		err = writeRuleItemString(writer, ruleItemProcessPathRegex, rule.ProcessPathRegex)
		if err != nil {
			return err
		}
	}
	if len(rule.PackageName) > 0 {
		err = writeRuleItemString(writer, ruleItemPackageName, rule.PackageName)
		if err != nil {
			return err
		}
	}
	if len(rule.NetworkType) > 0 {
		if generateVersion < C.RuleSetVersion3 {
			return E.New("`network_type` rule item is only supported in version 3 or later")
		}
		err = writeRuleItemUint8(writer, ruleItemNetworkType, rule.NetworkType)
		if err != nil {
			return err
		}
	}
	if rule.NetworkIsExpensive {
		if generateVersion < C.RuleSetVersion3 {
			return E.New("`network_is_expensive` rule item is only supported in version 3 or later")
		}
		err = binary.Write(writer, binary.BigEndian, ruleItemNetworkIsExpensive)
		if err != nil {
			return err
		}
	}
	if rule.NetworkIsConstrained {
		if generateVersion < C.RuleSetVersion3 {
			return E.New("`network_is_constrained` rule item is only supported in version 3 or later")
		}
		err = binary.Write(writer, binary.BigEndian, ruleItemNetworkIsConstrained)
		if err != nil {
			return err
		}
	}
	if rule.NetworkInterfaceAddress != nil && rule.NetworkInterfaceAddress.Size() > 0 {
		if generateVersion < C.RuleSetVersion4 {
			return E.New("`network_interface_address` rule item is only supported in version 4 or later")
		}
		err = writer.WriteByte(ruleItemNetworkInterfaceAddress)
		if err != nil {
			return err
		}
		_, err = varbin.WriteUvarint(writer, uint64(rule.NetworkInterfaceAddress.Size()))
		if err != nil {
			return err
		}
		for _, entry := range rule.NetworkInterfaceAddress.Entries() {
			err = binary.Write(writer, binary.BigEndian, uint8(entry.Key.Build()))
			if err != nil {
				return err
			}
			_, err = varbin.WriteUvarint(writer, uint64(len(entry.Value)))
			if err != nil {
				return err
			}
			for _, rawPrefix := range entry.Value {
				err = writePrefix(writer, rawPrefix.Build(netip.Prefix{}))
				if err != nil {
					return err
				}
			}
		}
	}
	if len(rule.DefaultInterfaceAddress) > 0 {
		if generateVersion < C.RuleSetVersion4 {
			return E.New("`default_interface_address` rule item is only supported in version 4 or later")
		}
		err = writer.WriteByte(ruleItemDefaultInterfaceAddress)
		if err != nil {
			return err
		}
		_, err = varbin.WriteUvarint(writer, uint64(len(rule.DefaultInterfaceAddress)))
		if err != nil {
			return err
		}
		for _, rawPrefix := range rule.DefaultInterfaceAddress {
			err = writePrefix(writer, rawPrefix.Build(netip.Prefix{}))
			if err != nil {
				return err
			}
		}
	}
	if len(rule.WIFISSID) > 0 {
		err = writeRuleItemString(writer, ruleItemWIFISSID, rule.WIFISSID)
		if err != nil {
			return err
		}
	}
	if len(rule.WIFIBSSID) > 0 {
		err = writeRuleItemString(writer, ruleItemWIFIBSSID, rule.WIFIBSSID)
		if err != nil {
			return err
		}
	}
	if len(rule.AdGuardDomain) > 0 {
		if generateVersion < C.RuleSetVersion2 {
			return E.New("AdGuard rule items is only supported in version 2 or later")
		}
		err = binary.Write(writer, binary.BigEndian, ruleItemAdGuardDomain)
		if err != nil {
			return err
		}
		err = domain.NewAdGuardMatcher(rule.AdGuardDomain).Write(writer)
		if err != nil {
			return err
		}
	}
	err = binary.Write(writer, binary.BigEndian, ruleItemFinal)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, rule.Invert)
	if err != nil {
		return err
	}
	return nil
}

func readRuleItemString(reader varbin.Reader) ([]string, error) {
	return varbin.ReadValue[[]string](reader, binary.BigEndian)
}

func writeRuleItemString(writer varbin.Writer, itemType uint8, value []string) error {
	err := writer.WriteByte(itemType)
	if err != nil {
		return err
	}
	return varbin.Write(writer, binary.BigEndian, value)
}

func readRuleItemUint8[E ~uint8](reader varbin.Reader) ([]E, error) {
	return varbin.ReadValue[[]E](reader, binary.BigEndian)
}

func writeRuleItemUint8[E ~uint8](writer varbin.Writer, itemType uint8, value []E) error {
	err := writer.WriteByte(itemType)
	if err != nil {
		return err
	}
	return varbin.Write(writer, binary.BigEndian, value)
}

func readRuleItemUint16(reader varbin.Reader) ([]uint16, error) {
	return varbin.ReadValue[[]uint16](reader, binary.BigEndian)
}

func writeRuleItemUint16(writer varbin.Writer, itemType uint8, value []uint16) error {
	err := writer.WriteByte(itemType)
	if err != nil {
		return err
	}
	return varbin.Write(writer, binary.BigEndian, value)
}

func writeRuleItemCIDR(writer varbin.Writer, itemType uint8, value []string) error {
	var builder netipx.IPSetBuilder
	for i, prefixString := range value {
		prefix, err := netip.ParsePrefix(prefixString)
		if err == nil {
			builder.AddPrefix(prefix)
			continue
		}
		addr, addrErr := netip.ParseAddr(prefixString)
		if addrErr == nil {
			builder.Add(addr)
			continue
		}
		return E.Cause(err, "parse [", i, "]")
	}
	ipSet, err := builder.IPSet()
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, itemType)
	if err != nil {
		return err
	}
	return writeIPSet(writer, ipSet)
}

func readLogicalRule(reader varbin.Reader, recovery bool) (logicalRule option.LogicalHeadlessRule, err error) {
	mode, err := reader.ReadByte()
	if err != nil {
		return
	}
	switch mode {
	case 0:
		logicalRule.Mode = C.LogicalTypeAnd
	case 1:
		logicalRule.Mode = C.LogicalTypeOr
	default:
		err = E.New("unknown logical mode: ", mode)
		return
	}
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	logicalRule.Rules = make([]option.HeadlessRule, length)
	for i := uint64(0); i < length; i++ {
		logicalRule.Rules[i], err = readRule(reader, recovery)
		if err != nil {
			err = E.Cause(err, "read logical rule [", i, "]")
			return
		}
	}
	err = binary.Read(reader, binary.BigEndian, &logicalRule.Invert)
	if err != nil {
		return
	}
	return
}

func writeLogicalRule(writer varbin.Writer, logicalRule option.LogicalHeadlessRule, generateVersion uint8) error {
	err := binary.Write(writer, binary.BigEndian, uint8(1))
	if err != nil {
		return err
	}
	switch logicalRule.Mode {
	case C.LogicalTypeAnd:
		err = binary.Write(writer, binary.BigEndian, uint8(0))
	case C.LogicalTypeOr:
		err = binary.Write(writer, binary.BigEndian, uint8(1))
	default:
		panic("unknown logical mode: " + logicalRule.Mode)
	}
	if err != nil {
		return err
	}
	_, err = varbin.WriteUvarint(writer, uint64(len(logicalRule.Rules)))
	if err != nil {
		return err
	}
	for _, rule := range logicalRule.Rules {
		err = writeRule(writer, rule, generateVersion)
		if err != nil {
			return err
		}
	}
	err = binary.Write(writer, binary.BigEndian, logicalRule.Invert)
	if err != nil {
		return err
	}
	return nil
}

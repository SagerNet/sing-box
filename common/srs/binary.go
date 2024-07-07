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
	ruleItemFinal uint8 = 0xFF
)

func Read(reader io.Reader, recover bool) (ruleSet option.PlainRuleSet, err error) {
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
		return ruleSet, err
	}
	if version != 1 {
		return ruleSet, E.New("unsupported version: ", version)
	}
	zReader, err := zlib.NewReader(reader)
	if err != nil {
		return
	}
	bReader := bufio.NewReader(zReader)
	length, err := binary.ReadUvarint(bReader)
	if err != nil {
		return
	}
	ruleSet.Rules = make([]option.HeadlessRule, length)
	for i := uint64(0); i < length; i++ {
		ruleSet.Rules[i], err = readRule(bReader, recover)
		if err != nil {
			err = E.Cause(err, "read rule[", i, "]")
			return
		}
	}
	return
}

func Write(writer io.Writer, ruleSet option.PlainRuleSet) error {
	_, err := writer.Write(MagicBytes[:])
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, uint8(1))
	if err != nil {
		return err
	}
	zWriter, err := zlib.NewWriterLevel(writer, zlib.BestCompression)
	if err != nil {
		return err
	}
	bWriter := bufio.NewWriter(zWriter)
	_, err = varbin.WriteUvarint(bWriter, uint64(len(ruleSet.Rules)))
	if err != nil {
		return err
	}
	for _, rule := range ruleSet.Rules {
		err = writeRule(bWriter, rule)
		if err != nil {
			return err
		}
	}
	err = bWriter.Flush()
	if err != nil {
		return err
	}
	return zWriter.Close()
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

func writeRule(writer varbin.Writer, rule option.HeadlessRule) error {
	switch rule.Type {
	case C.RuleTypeDefault:
		return writeDefaultRule(writer, rule.DefaultOptions)
	case C.RuleTypeLogical:
		return writeLogicalRule(writer, rule.LogicalOptions)
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
		case ruleItemPackageName:
			rule.PackageName, err = readRuleItemString(reader)
		case ruleItemWIFISSID:
			rule.WIFISSID, err = readRuleItemString(reader)
		case ruleItemWIFIBSSID:
			rule.WIFIBSSID, err = readRuleItemString(reader)
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

func writeDefaultRule(writer varbin.Writer, rule option.DefaultHeadlessRule) error {
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
		err = domain.NewMatcher(rule.Domain, rule.DomainSuffix).Write(writer)
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
	if len(rule.PackageName) > 0 {
		err = writeRuleItemString(writer, ruleItemPackageName, rule.PackageName)
		if err != nil {
			return err
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

func writeLogicalRule(writer varbin.Writer, logicalRule option.LogicalHeadlessRule) error {
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
		err = writeRule(writer, rule)
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

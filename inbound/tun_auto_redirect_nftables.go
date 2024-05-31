//go:build linux

package inbound

import (
	"net/netip"

	"github.com/sagernet/nftables"
	"github.com/sagernet/nftables/binaryutil"
	"github.com/sagernet/nftables/expr"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/sys/unix"
)

const (
	nftablesTableName       = "sing-box"
	nftablesChainOutput     = "output"
	nftablesChainForward    = "forward"
	nftablesChainPreRouting = "prerouting"
)

func nftablesFamily(family int) nftables.TableFamily {
	switch family {
	case unix.AF_INET:
		return nftables.TableFamilyIPv4
	case unix.AF_INET6:
		return nftables.TableFamilyIPv6
	default:
		panic(F.ToString("unknown family ", family))
	}
}

func (t *tunAutoRedirect) setupNfTables(family int) error {
	nft, err := nftables.New()
	if err != nil {
		return err
	}
	defer nft.CloseLasting()
	table := nft.AddTable(&nftables.Table{
		Name:   nftablesTableName,
		Family: nftablesFamily(family),
	})
	chainOutput := nft.AddChain(&nftables.Chain{
		Name:     nftablesChainOutput,
		Table:    table,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityMangle,
		Type:     nftables.ChainTypeNAT,
	})
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chainOutput,
		Exprs: nftablesRuleIfName(expr.MetaKeyOIFNAME, t.tunOptions.Name, nftablesRuleRedirectToPorts(M.AddrPortFromNet(t.tcpListener.Addr()).Port())...),
	})
	chainForward := nft.AddChain(&nftables.Chain{
		Name:     nftablesChainForward,
		Table:    table,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityMangle,
	})
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chainForward,
		Exprs: nftablesRuleIfName(expr.MetaKeyIIFNAME, t.tunOptions.Name, &expr.Verdict{
			Kind: expr.VerdictAccept,
		}),
	})
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chainForward,
		Exprs: nftablesRuleIfName(expr.MetaKeyOIFNAME, t.tunOptions.Name, &expr.Verdict{
			Kind: expr.VerdictAccept,
		}),
	})
	t.setupNfTablesPreRouting(nft, table)
	return nft.Flush()
}

func (t *tunAutoRedirect) setupNfTablesPreRouting(nft *nftables.Conn, table *nftables.Table) {
	chainPreRouting := nft.AddChain(&nftables.Chain{
		Name:     nftablesChainPreRouting,
		Table:    table,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityMangle,
		Type:     nftables.ChainTypeNAT,
	})
	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chainPreRouting,
		Exprs: nftablesRuleIfName(expr.MetaKeyIIFNAME, t.tunOptions.Name, &expr.Verdict{
			Kind: expr.VerdictReturn,
		}),
	})
	var (
		routeAddress        []netip.Prefix
		routeExcludeAddress []netip.Prefix
	)
	if table.Family == nftables.TableFamilyIPv4 {
		routeAddress = t.tunOptions.Inet4RouteAddress
		routeExcludeAddress = t.tunOptions.Inet4RouteExcludeAddress
	} else {
		routeAddress = t.tunOptions.Inet6RouteAddress
		routeExcludeAddress = t.tunOptions.Inet6RouteExcludeAddress
	}
	for _, address := range routeExcludeAddress {
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chainPreRouting,
			Exprs: nftablesRuleDestinationAddress(address, &expr.Verdict{
				Kind: expr.VerdictReturn,
			}),
		})
	}
	for _, name := range t.tunOptions.ExcludeInterface {
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chainPreRouting,
			Exprs: nftablesRuleIfName(expr.MetaKeyIIFNAME, name, &expr.Verdict{
				Kind: expr.VerdictReturn,
			}),
		})
	}
	for _, uidRange := range t.tunOptions.ExcludeUID {
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chainPreRouting,
			Exprs: nftablesRuleMetaUInt32Range(expr.MetaKeySKUID, uidRange, &expr.Verdict{
				Kind: expr.VerdictReturn,
			}),
		})
	}

	var routeExprs []expr.Any
	if len(routeAddress) > 0 {
		for _, address := range routeAddress {
			routeExprs = append(routeExprs, nftablesRuleDestinationAddress(address)...)
		}
	}
	redirectPort := M.AddrPortFromNet(t.tcpListener.Addr()).Port()
	var dnsServerAddress netip.Addr
	if table.Family == nftables.TableFamilyIPv4 {
		dnsServerAddress = t.tunOptions.Inet4Address[0].Addr().Next()
	} else {
		dnsServerAddress = t.tunOptions.Inet6Address[0].Addr().Next()
	}

	if len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0 {
		for _, name := range t.tunOptions.IncludeInterface {
			nft.AddRule(&nftables.Rule{
				Table: table,
				Chain: chainPreRouting,
				Exprs: nftablesRuleIfName(expr.MetaKeyIIFNAME, name, append(routeExprs, nftablesRuleHijackDNS(table.Family, dnsServerAddress)...)...),
			})
		}
		for _, uidRange := range t.tunOptions.IncludeUID {
			nft.AddRule(&nftables.Rule{
				Table: table,
				Chain: chainPreRouting,
				Exprs: nftablesRuleMetaUInt32Range(expr.MetaKeySKUID, uidRange, append(routeExprs, nftablesRuleHijackDNS(table.Family, dnsServerAddress)...)...),
			})
		}
	} else {
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chainPreRouting,
			Exprs: append(routeExprs, nftablesRuleHijackDNS(table.Family, dnsServerAddress)...),
		})
	}

	nft.AddRule(&nftables.Rule{
		Table: table,
		Chain: chainPreRouting,
		Exprs: []expr.Any{
			&expr.Fib{
				Register:       1,
				FlagDADDR:      true,
				ResultADDRTYPE: true,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     binaryutil.NativeEndian.PutUint32(unix.RTN_LOCAL),
			},
			&expr.Verdict{
				Kind: expr.VerdictReturn,
			},
		},
	})

	if len(t.tunOptions.IncludeInterface) > 0 || len(t.tunOptions.IncludeUID) > 0 {
		for _, name := range t.tunOptions.IncludeInterface {
			nft.AddRule(&nftables.Rule{
				Table: table,
				Chain: chainPreRouting,
				Exprs: nftablesRuleIfName(expr.MetaKeyIIFNAME, name, append(routeExprs, nftablesRuleRedirectToPorts(redirectPort)...)...),
			})
		}
		for _, uidRange := range t.tunOptions.IncludeUID {
			nft.AddRule(&nftables.Rule{
				Table: table,
				Chain: chainPreRouting,
				Exprs: nftablesRuleMetaUInt32Range(expr.MetaKeySKUID, uidRange, append(routeExprs, nftablesRuleRedirectToPorts(redirectPort)...)...),
			})
		}
	} else {
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: chainPreRouting,
			Exprs: append(routeExprs, nftablesRuleRedirectToPorts(redirectPort)...),
		})
	}
}

func (t *tunAutoRedirect) cleanupNfTables(family int) {
	conn, err := nftables.New()
	if err != nil {
		return
	}
	defer conn.CloseLasting()
	conn.FlushTable(&nftables.Table{
		Name:   nftablesTableName,
		Family: nftablesFamily(family),
	})
	conn.DelTable(&nftables.Table{
		Name:   nftablesTableName,
		Family: nftablesFamily(family),
	})
	_ = conn.Flush()
}

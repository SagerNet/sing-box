package deprecated

import (
	"fmt"

	"github.com/sagernet/sing-box/common/badversion"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/locale"
	F "github.com/sagernet/sing/common/format"

	"golang.org/x/mod/semver"
)

type Note struct {
	Name              string
	Description       string
	DeprecatedVersion string
	ScheduledVersion  string
	EnvName           string
	MigrationLink     string
}

func (n Note) Impending() bool {
	if n.ScheduledVersion == "" {
		return false
	}
	if !semver.IsValid("v" + C.Version) {
		return false
	}
	versionCurrent := badversion.Parse(C.Version)
	versionMinor := badversion.Parse(n.ScheduledVersion).Minor - versionCurrent.Minor
	if versionCurrent.PreReleaseIdentifier == "" && versionMinor < 0 {
		panic("invalid deprecated note: " + n.Name)
	}
	return versionMinor <= 1
}

func (n Note) Message() string {
	if n.MigrationLink != "" {
		return fmt.Sprintf(locale.Current().DeprecatedMessage, n.Description, n.DeprecatedVersion, n.ScheduledVersion)
	} else {
		return fmt.Sprintf(locale.Current().DeprecatedMessageNoLink, n.Description, n.DeprecatedVersion, n.ScheduledVersion)
	}
}

func (n Note) MessageWithLink() string {
	if n.MigrationLink != "" {
		return F.ToString(
			n.Description, " is deprecated in sing-box ", n.DeprecatedVersion,
			" and will be removed in sing-box ", n.ScheduledVersion, ", checkout documentation for migration: ", n.MigrationLink,
		)
	} else {
		return F.ToString(
			n.Description, " is deprecated in sing-box ", n.DeprecatedVersion,
			" and will be removed in sing-box ", n.ScheduledVersion, ".",
		)
	}
}

var OptionBadMatchSource = Note{
	Name:              "bad-match-source",
	Description:       "legacy match source rule item",
	DeprecatedVersion: "1.10.0",
	ScheduledVersion:  "1.11.0",
	EnvName:           "BAD_MATCH_SOURCE",
	MigrationLink:     "https://sing-box.sagernet.org/deprecated/#match-source-rule-items-are-renamed",
}

var OptionGEOIP = Note{
	Name:              "geoip",
	Description:       "geoip database",
	DeprecatedVersion: "1.8.0",
	ScheduledVersion:  "1.12.0",
	EnvName:           "GEOIP",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-geoip-to-rule-sets",
}

var OptionGEOSITE = Note{
	Name:              "geosite",
	Description:       "geosite database",
	DeprecatedVersion: "1.8.0",
	ScheduledVersion:  "1.12.0",
	EnvName:           "GEOSITE",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-geosite-to-rule-sets",
}

var OptionTUNAddressX = Note{
	Name:              "tun-address-x",
	Description:       "legacy tun address fields",
	DeprecatedVersion: "1.10.0",
	ScheduledVersion:  "1.12.0",
	EnvName:           "TUN_ADDRESS_X",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#tun-address-fields-are-merged",
}

var OptionSpecialOutbounds = Note{
	Name:              "special-outbounds",
	Description:       "legacy special outbounds",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "SPECIAL_OUTBOUNDS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-legacy-special-outbounds-to-rule-actions",
}

var OptionInboundOptions = Note{
	Name:              "inbound-options",
	Description:       "legacy inbound fields",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "INBOUND_OPTIONS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-legacy-special-outbounds-to-rule-actions",
}

var OptionDestinationOverrideFields = Note{
	Name:              "destination-override-fields",
	Description:       "destination override fields in direct outbound",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "DESTINATION_OVERRIDE_FIELDS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-destination-override-fields-to-route-options",
}

var OptionWireGuardOutbound = Note{
	Name:              "wireguard-outbound",
	Description:       "legacy wireguard outbound",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "WIREGUARD_OUTBOUND",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-wireguard-outbound-to-endpoint",
}

var OptionWireGuardGSO = Note{
	Name:              "wireguard-gso",
	Description:       "GSO option in wireguard outbound",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "WIREGUARD_GSO",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-wireguard-outbound-to-endpoint",
}

var OptionTUNGSO = Note{
	Name:              "tun-gso",
	Description:       "GSO option in tun",
	DeprecatedVersion: "1.11.0",
	ScheduledVersion:  "1.12.0",
	EnvName:           "TUN_GSO",
	MigrationLink:     "https://sing-box.sagernet.org/deprecated/#gso-option-in-tun",
}

var OptionLegacyDNSTransport = Note{
	Name:              "legacy-dns-transport",
	Description:       "legacy DNS servers",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "LEGACY_DNS_SERVERS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-to-new-dns-server-formats",
}

var OptionLegacyDNSFakeIPOptions = Note{
	Name:              "legacy-dns-fakeip-options",
	Description:       "legacy DNS fakeip options",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "LEGACY_DNS_FAKEIP_OPTIONS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-to-new-dns-server-formats",
}

var OptionOutboundDNSRuleItem = Note{
	Name:              "outbound-dns-rule-item",
	Description:       "outbound DNS rule item",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "OUTBOUND_DNS_RULE_ITEM",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-outbound-dns-rule-items-to-domain-resolver",
}

var OptionMissingDomainResolver = Note{
	Name:              "missing-domain-resolver",
	Description:       "missing `route.default_domain_resolver` or `domain_resolver` in dial fields",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "MISSING_DOMAIN_RESOLVER",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-outbound-dns-rule-items-to-domain-resolver",
}

var OptionLegacyECHOptions = Note{
	Name:              "legacy-ech-options",
	Description:       "legacy ECH options",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.13.0",
	EnvName:           "LEGACY_ECH_OPTIONS",
	MigrationLink:     "https://sing-box.sagernet.org/deprecated/#legacy-ech-fields",
}

var OptionLegacyDomainStrategyOptions = Note{
	Name:              "legacy-domain-strategy-options",
	Description:       "legacy domain strategy options",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "LEGACY_DOMAIN_STRATEGY_OPTIONS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-domain-strategy-options",
}

var Options = []Note{
	OptionBadMatchSource,
	OptionGEOIP,
	OptionGEOSITE,
	OptionTUNAddressX,
	OptionSpecialOutbounds,
	OptionInboundOptions,
	OptionDestinationOverrideFields,
	OptionWireGuardOutbound,
	OptionWireGuardGSO,
	OptionTUNGSO,
	OptionLegacyDNSTransport,
	OptionLegacyDNSFakeIPOptions,
	OptionOutboundDNSRuleItem,
	OptionMissingDomainResolver,
	OptionLegacyECHOptions,
	OptionLegacyDomainStrategyOptions,
}

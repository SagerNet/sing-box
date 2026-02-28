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

var OptionLegacyDomainStrategyOptions = Note{
	Name:              "legacy-domain-strategy-options",
	Description:       "legacy domain strategy options",
	DeprecatedVersion: "1.12.0",
	ScheduledVersion:  "1.14.0",
	EnvName:           "LEGACY_DOMAIN_STRATEGY_OPTIONS",
	MigrationLink:     "https://sing-box.sagernet.org/migration/#migrate-domain-strategy-options",
}

var Options = []Note{
	OptionLegacyDNSTransport,
	OptionLegacyDNSFakeIPOptions,
	OptionOutboundDNSRuleItem,
	OptionMissingDomainResolver,
	OptionLegacyDomainStrategyOptions,
}

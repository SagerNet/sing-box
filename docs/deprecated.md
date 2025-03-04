---
icon: material/delete-alert
---

# Deprecated Feature List

## 1.12.0

#### Legacy DNS server formats

DNS servers are refactored,
check [Migration](../migration/#migrate-to-new-dns-servers).

Compatibility for old formats will be removed in sing-box 1.14.0.

#### `outbound` DNS rule item

Legacy `outbound` DNS rules are deprecated
and can be replaced by dial fields,
check [Migration](../migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

#### Legacy ECH fields

ECH support has been migrated to use stdlib in sing-box 1.12.0,
which does not come with support for PQ signature schemes,
so `pq_signature_schemes_enabled` has been deprecated and no longer works.

Also, `dynamic_record_sizing_disabled` has nothing to do with ECH,
was added by mistake, has been deprecated and no longer works.

These fields will be removed in sing-box 1.13.0.

## 1.11.0

#### Legacy special outbounds

Legacy special outbounds (`block` / `dns`) are deprecated
and can be replaced by rule actions,
check [Migration](../migration/#migrate-legacy-special-outbounds-to-rule-actions).

Old fields will be removed in sing-box 1.13.0.

#### Legacy inbound fields

Legacy inbound fields （`inbound.<sniff/domain_strategy/...>` are deprecated
and can be replaced by rule actions,
check [Migration](../migration/#migrate-legacy-inbound-fields-to-rule-actions).

Old fields will be removed in sing-box 1.13.0.

#### Destination override fields in direct outbound

Destination override fields (`override_address` / `override_port`) in direct outbound are deprecated
and can be replaced by rule actions,
check [Migration](../migration/#migrate-destination-override-fields-to-route-options).

#### WireGuard outbound

WireGuard outbound is deprecated and can be replaced by endpoint,
check [Migration](../migration/#migrate-wireguard-outbound-to-endpoint).

Old outbound will be removed in sing-box 1.13.0.

#### GSO option in TUN

GSO has no advantages for transparent proxy scenarios, is deprecated and no longer works in TUN.

Old fields will be removed in sing-box 1.13.0.

## 1.10.0

#### TUN address fields are merged

`inet4_address` and `inet6_address` are merged into `address`,
`inet4_route_address` and `inet6_route_address` are merged into `route_address`,
`inet4_route_exclude_address` and `inet6_route_exclude_address` are merged into `route_exclude_address`.

Old fields will be removed in sing-box 1.12.0.

#### Match source rule items are renamed

`rule_set_ipcidr_match_source` route and DNS rule items are renamed to
`rule_set_ip_cidr_match_source` and will be remove in sing-box 1.11.0.

#### Drop support for go1.18 and go1.19

Due to maintenance difficulties, sing-box 1.10.0 requires at least Go 1.20 to compile.

## 1.8.0

#### Cache file and related features in Clash API

`cache_file` and related features in Clash API is migrated to independent `cache_file` options,
check [Migration](/migration/#migrate-cache-file-from-clash-api-to-independent-options).

#### GeoIP

GeoIP is deprecated and will be removed in sing-box 1.12.0.

The maxmind GeoIP National Database, as an IP classification database,
is not entirely suitable for traffic bypassing,
and all existing implementations suffer from high memory usage and difficult management.

sing-box 1.8.0 introduces [rule-set](/configuration/rule-set/), which can completely replace GeoIP,
check [Migration](/migration/#migrate-geoip-to-rule-sets).

#### Geosite

Geosite is deprecated and will be removed in sing-box 1.12.0.

Geosite, the `domain-list-community` project maintained by V2Ray as an early traffic bypassing solution,
suffers from a number of problems, including lack of maintenance, inaccurate rules, and difficult management.

sing-box 1.8.0 introduces [rule-set](/configuration/rule-set/), which can completely replace Geosite,
check [Migration](/migration/#migrate-geosite-to-rule-sets).

## 1.6.0

The following features will be marked deprecated in 1.5.0 and removed entirely in 1.6.0.

#### ShadowsocksR

ShadowsocksR support has never been enabled by default, since the most commonly used proxy sales panel in the
illegal industry stopped using this protocol, it does not make sense to continue to maintain it.

#### Proxy Protocol

Proxy Protocol is added by Pull Request, has problems, is only used by the backend of HTTP multiplexers such as nginx,
is intrusive, and is meaningless for proxy purposes.

---
icon: material/delete-alert
---

# Deprecated Feature List

## 1.8.0

#### Cache file and related features in Clash API

`cache_file` and related features in Clash API is migrated to independent `cache_file` options,
check [Migration](/migration/#migrate-cache-file-from-clash-api-to-independent-options).

#### GeoIP

GeoIP is deprecated and may be removed in the future.

The maxmind GeoIP National Database, as an IP classification database,
is not entirely suitable for traffic bypassing,
and all existing implementations suffer from high memory usage and difficult management.

sing-box 1.8.0 introduces [Rule Set](/configuration/rule_set), which can completely replace GeoIP,
check [Migration](/migration/#migrate-geoip-to-rule-sets).

#### Geosite

Geosite is deprecated and may be removed in the future.

Geosite, the `domain-list-community` project maintained by V2Ray as an early traffic bypassing solution,
suffers from a number of problems, including lack of maintenance, inaccurate rules, and difficult management.

sing-box 1.8.0 introduces [Rule Set](/configuration/rule_set), which can completely replace Geosite,
check [Migration](/migration/#migrate-geosite-to-rule-sets).

Geosite，即由 V2Ray 维护的 domain-list-community 项目，作为早期流量绕过解决方案，存在着大量问题，包括缺少维护、规则不准确、管理困难。

## 1.6.0

The following features will be marked deprecated in 1.5.0 and removed entirely in 1.6.0.

#### ShadowsocksR

ShadowsocksR support has never been enabled by default, since the most commonly used proxy sales panel in the
illegal industry stopped using this protocol, it does not make sense to continue to maintain it.

#### Proxy Protocol

Proxy Protocol is added by Pull Request, has problems, is only used by the backend of HTTP multiplexers such as nginx,
is intrusive, and is meaningless for proxy purposes.

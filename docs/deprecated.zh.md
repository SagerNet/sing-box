---
icon: material/delete-alert
---

# 废弃功能列表

#### 旧的 DNS 服务器格式

DNS 服务器已重构，
参阅 [迁移指南](/migration/#migrate-to-new-dns-servers).

对旧格式的兼容性将在 sing-box 1.14.0 中被移除。

#### `outbound` DNS 规则项

旧的 `outbound` DNS 规则已废弃，
且可被拨号字段代替，
参阅 [迁移指南](/migration/#migrate-outbound-dns-rule-items-to-domain-resolver).

#### 旧的 ECH 字段

ECH 支持已在 sing-box 1.12.0 迁移至使用标准库，但标准库不支持后量子对等证书签名方案，
因此 `pq_signature_schemes_enabled` 已被弃用且不再工作。

另外，`dynamic_record_sizing_disabled` 与 ECH 无关，是错误添加的，现已弃用且不再工作。

相关字段将在 sing-box 1.13.0 中被移除。

## 1.11.0

#### 旧的特殊出站

旧的特殊出站（`block` / `dns`）已废弃且可以通过规则动作替代，
参阅 [迁移指南](/migration/#migrate-legacy-special-outbounds-to-rule-actions)。

旧字段将在 sing-box 1.13.0 中被移除。

#### 旧的入站字段

旧的入站字段（`inbound.<sniff/domain_strategy/...>`）已废弃且可以通过规则动作替代，
参阅 [迁移指南](/migration/#migrate-legacy-inbound-fields-to-rule-actions)。

旧字段将在 sing-box 1.13.0 中被移除。

#### direct 出站中的目标地址覆盖字段

direct 出站中的目标地址覆盖字段（`override_address` / `override_port`）已废弃且可以通过规则动作替代，
参阅 [迁移指南](/migration/#migrate-destination-override-fields-to-route-options)。

旧字段将在 sing-box 1.13.0 中被移除。

#### WireGuard 出站

WireGuard 出站已废弃且可以通过端点替代，
参阅 [迁移指南](/migration/#migrate-wireguard-outbound-to-endpoint)。

旧出站将在 sing-box 1.13.0 中被移除。

#### TUN 的 GSO 字段

GSO 对透明代理场景没有优势，已废弃且在 TUN 中不再起作用。

旧字段将在 sing-box 1.13.0 中被移除。

## 1.10.0

#### Match source 规则项已重命名

`rule_set_ipcidr_match_source` 路由和 DNS 规则项已被重命名为
`rule_set_ip_cidr_match_source` 且将在 sing-box 1.11.0 中被移除。

#### TUN 地址字段已合并

`inet4_address` 和 `inet6_address` 已合并为 `address`，
`inet4_route_address` 和 `inet6_route_address` 已合并为 `route_address`，
`inet4_route_exclude_address` 和 `inet6_route_exclude_address` 已合并为 `route_exclude_address`。

旧字段将在 sing-box 1.11.0 中被移除。

#### 移除对 go1.18 和 go1.19 的支持

由于维护困难，sing-box 1.10.0 要求至少 Go 1.20 才能编译。

## 1.8.0

#### Clash API 中的 Cache file 及相关功能

Clash API 中的 `cache_file` 及相关功能已废弃且已迁移到独立的 `cache_file` 设置，
参阅 [迁移指南](/zh/migration/#clash-api)。

#### GeoIP

GeoIP 已废弃且将在 sing-box 1.12.0 中被移除。

maxmind GeoIP 国家数据库作为 IP 分类数据库，不完全适合流量绕过，
且现有的实现均存在内存使用大与管理困难的问题。

sing-box 1.8.0 引入了[规则集](/configuration/rule-set/)，
可以完全替代 GeoIP， 参阅 [迁移指南](/zh/migration/#geoip)。

#### Geosite

Geosite 已废弃且将在 sing-box 1.12.0 中被移除。

Geosite，即由 V2Ray 维护的 domain-list-community 项目，作为早期流量绕过解决方案，
存在着包括缺少维护、规则不准确和管理困难内的大量问题。

sing-box 1.8.0 引入了[规则集](/configuration/rule-set/)，
可以完全替代 Geosite，参阅 [迁移指南](/zh/migration/#geosite)。

## 1.6.0

下列功能已在 1.5.0 中标记为已弃用，并在 1.6.0 中完全删除。

#### ShadowsocksR

ShadowsocksR 支持从未默认启用，自从常用的黑产代理销售面板停止使用该协议，继续维护它是没有意义的。

#### Proxy Protocol

Proxy Protocol 支持由 Pull Request 添加，存在问题且仅由 HTTP 多路复用器（如 nginx）的后端使用，具有侵入性，对于代理目的毫无意义。

---
icon: material/delete-alert
---

# 废弃功能列表

## 1.8.0

#### Clash API 中的 Cache file 及相关功能

Clash API 中的 `cache_file` 及相关功能已废弃且已迁移到独立的 `cache_file` 设置，
参阅 [迁移指南](/zh/migration/#clash-api)。

#### GeoIP

GeoIP 已废弃且可能在不久的将来移除。

maxmind GeoIP 国家数据库作为 IP 分类数据库，不完全适合流量绕过，
且现有的实现均存在内存使用大与管理困难的问题。

sing-box 1.8.0 引入了[规则集](/configuration/rule_set)，
可以完全替代 GeoIP， 参阅 [迁移指南](/zh/migration/#geoip)。

#### Geosite

Geosite 已废弃且可能在不久的将来移除。

Geosite，即由 V2Ray 维护的 domain-list-community 项目，作为早期流量绕过解决方案，
存在着包括缺少维护、规则不准确和管理困难内的大量问题。

sing-box 1.8.0 引入了[规则集](/configuration/rule_set)，
可以完全替代 Geosite，参阅 [迁移指南](/zh/migration/#geosite)。

## 1.6.0

下列功能已在 1.5.0 中标记为已弃用，并在 1.6.0 中完全删除。

#### ShadowsocksR

ShadowsocksR 支持从未默认启用，自从常用的黑产代理销售面板停止使用该协议，继续维护它是没有意义的。

#### Proxy Protocol

Proxy Protocol 支持由 Pull Request 添加，存在问题且仅由 HTTP 多路复用器（如 nginx）的后端使用，具有侵入性，对于代理目的毫无意义。

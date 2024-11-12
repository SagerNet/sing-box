---
icon: material/new-box
---

## 最终动作

### route

```json
{
  "action": "route", // 默认
  "outbound": "",
  "network_strategy": "",
  "fallback_delay": "",
  "udp_disable_domain_unmapping": false,
  "udp_connect": false
}
```

`route` 继承了将连接路由到指定出站的经典规则动作。

#### outbound

==必填==

目标出站的标签。

#### network_strategy

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持，并且需要 `auto_detect_interface`。

选择网络接口的策略。

仅当出站为 `direct` 且 `outbound.bind_interface`, `outbound.inet4_bind_address`
且 `outbound.inet6_bind_address` 未设置时生效。

可用值参阅 [拨号字段](/configuration/shared/dial/#network_strategy)。

#### fallback_delay

!!! quote ""

    仅在 Android 与 Apple 平台图形客户端中支持，并且需要 `auto_detect_interface` 且 `network_strategy` 已设置。

详情参阅 [拨号字段](/configuration/shared/dial/#fallback_delay)。

#### udp_disable_domain_unmapping

如果启用，对于地址为域的 UDP 代理请求，将在响应中发送原始包地址而不是映射的域。

此选项用于兼容不支持接收带有域地址的 UDP 包的客户端，如 Surge。

#### udp_connect

如果启用，将尝试将 UDP 连接 connect 到目标而不是 listen。

### route-options

```json
{
  "action": "route-options",
  "network_strategy": "",
  "fallback_delay": "",
  "udp_disable_domain_unmapping": false,
  "udp_connect": false
}
```

`route-options` 为路由设置选项。

### reject

```json
{
  "action": "reject",
  "method": "default",  // 默认
  "no_drop": false
}
```

`reject` 拒绝连接。

如果尚未执行 `sniff` 操作，则将使用指定方法拒绝 tun 连接。

对于非 tun 连接和已建立的连接，将直接关闭。

#### method

- `default`: 对于 TCP 连接回复 RST，对于 UDP 包回复 ICMP 端口不可达。
- `drop`: 丢弃数据包。

#### no_drop

如果未启用，则 30 秒内触发 50 次后，`method` 将被暂时覆盖为 `drop`。

当 `method` 设为 `drop` 时不可用。

### hijack-dns

```json
{
  "action": "hijack-dns"
}
```

`hijack-dns` 劫持 DNS 请求至 sing-box DNS 模块。

## 非最终动作

### sniff

```json
{
  "action": "sniff",
  "sniffer": [],
  "timeout": ""
}
```

`sniff` 对连接执行协议嗅探。

对于已弃用的 `inbound.sniff` 选项，被视为在路由之前执行的 `sniff`。

#### sniffer

启用的探测器。

默认启用所有探测器。

可用的协议值可以在 [协议嗅探](../sniff/) 中找到。

#### timeout

探测超时时间。

默认使用 300ms。

### resolve

```json
{
  "action": "resolve",
  "strategy": "",
  "server": ""
}
```

`resolve` 将请求的目标从域名解析为 IP 地址。

#### strategy

DNS 解析策略，可用值有：`prefer_ipv4`、`prefer_ipv6`、`ipv4_only`、`ipv6_only`。

默认使用 `dns.strategy`。

#### server

指定要使用的 DNS 服务器的标签，而不是通过 DNS 路由进行选择。

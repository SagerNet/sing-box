---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.12.0"

    :material-plus: [netns](#netns)  
    :material-plus: [bind_interface](#bind_interface)  
    :material-plus: [routing_mark](#routing_mark)  
    :material-plus: [reuse_addr](#reuse_addr)

!!! quote "sing-box 1.11.0 中的更改"

    :material-delete-clock: [sniff](#sniff)  
    :material-delete-clock: [sniff_override_destination](#sniff_override_destination)  
    :material-delete-clock: [sniff_timeout](#sniff_timeout)  
    :material-delete-clock: [domain_strategy](#domain_strategy)  
    :material-delete-clock: [udp_disable_domain_unmapping](#udp_disable_domain_unmapping)

### 结构

```json
{
  "listen": "",
  "listen_port": 0,
  "bind_interface": "",
  "routing_mark": 0,
  "reuse_addr": false,
  "netns": "",
  "tcp_fast_open": false,
  "tcp_multi_path": false,
  "udp_fragment": false,
  "udp_timeout": "",
  "detour": "",

  // 废弃的
  
  "sniff": false,
  "sniff_override_destination": false,
  "sniff_timeout": "",
  "domain_strategy": "",
  "udp_disable_domain_unmapping": false
}
```

### 字段

#### listen

==必填==

监听地址。

#### listen_port

监听端口。

#### bind_interface

!!! question "自 sing-box 1.12.0 起"

要绑定到的网络接口。

#### routing_mark

!!! question "自 sing-box 1.12.0 起"

!!! quote ""

    仅支持 Linux。

设置 netfilter 路由标记。

支持数字 (如 `1234`) 和十六进制字符串 (如 `"0x1234"`)。

#### reuse_addr

!!! question "自 sing-box 1.12.0 起"

重用监听地址。

#### netns

!!! question "自 sing-box 1.12.0 起"

!!! quote ""

    仅支持 Linux。

设置网络命名空间，名称或路径。

#### tcp_fast_open

启用 TCP Fast Open。

#### tcp_multi_path

!!! warning ""

    需要 Go 1.21。

启用 TCP Multi Path。

#### udp_fragment

启用 UDP 分段。

#### udp_timeout

UDP NAT 过期时间。

默认使用 `5m`。

#### detour

如果设置，连接将被转发到指定的入站。

需要目标入站支持，参阅 [注入支持](/zh/configuration/inbound/#_3)。

#### sniff

!!! failure "已在 sing-box 1.11.0 废弃"

    入站字段已废弃且将在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

启用协议探测。

参阅 [协议探测](/zh/configuration/route/sniff/)

#### sniff_override_destination

!!! failure "已在 sing-box 1.11.0 废弃"

    入站字段已废弃且将在 sing-box 1.12.0 中被移除。

用探测出的域名覆盖连接目标地址。

如果域名无效（如 Tor），将不生效。

#### sniff_timeout

!!! failure "已在 sing-box 1.11.0 废弃"

    入站字段已废弃且将在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

探测超时时间。

默认使用 300ms。

#### domain_strategy

!!! failure "已在 sing-box 1.11.0 废弃"

    入站字段已废弃且将在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

可选值： `prefer_ipv4` `prefer_ipv6` `ipv4_only` `ipv6_only`。

如果设置，请求的域名将在路由之前解析为 IP。

如果 `sniff_override_destination` 生效，它的值将作为后备。

#### udp_disable_domain_unmapping

!!! failure "已在 sing-box 1.11.0 废弃"

    入站字段已废弃且将在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/migration/#migrate-legacy-inbound-fields-to-rule-actions).

如果启用，对于地址为域的 UDP 代理请求，将在响应中发送原始包地址而不是映射的域。

此选项用于兼容不支持接收带有域地址的 UDP 包的客户端，如 Surge。

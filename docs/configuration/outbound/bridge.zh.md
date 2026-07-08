---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Bridge

!!! quote ""

    需要特权。支持 Linux、macOS、rooted Android 和越狱 iOS。

    对于图形客户端：macOS 仅独立版本可用，且需要 Root Helper；Android 需要 root 权限；iOS 需要越狱。

`bridge` 是 `direct` 出站的 L3 版本：它将 L3 连接（TCP、UDP 和 ICMP）直接从网络接口转发出去。
通过[预匹配](/zh/configuration/shared/pre-match/)中的 `route` 动作，将 L3 流量从 TUN
或其他 L3 endpoints 路由到它；L4 连接将被拒绝。

到本机本地地址（loopback 或分配给本机网络接口的地址）的流量将被拒绝。

建议使用 [`preferred_by`](/zh/configuration/route/rule/#preferred_by) 作为 `route`
规则的门禁：它仅在[预匹配](/zh/configuration/shared/pre-match/)中匹配，且排除了无法路由的本地地址。

### 结构

```json
{
  "type": "bridge",
  "tag": "bridge-out",

  "interface": "",
  "bridge_name": "",
  "iproute2_table_index": 0,
  "iproute2_rule_index": 0
}
```

### 字段

#### interface

转发流量流出的网络接口名称。

默认使用默认接口。

接口不可用期间，转发流量将被丢弃。

#### bridge_name

自定义 bridge TUN 接口名前缀，默认使用 `bridge`。

在 Apple 平台上无效。

#### iproute2_table_index

!!! quote ""

    仅支持 Linux，且仅在设置了 `interface` 时生效。

用于固定出口路由的 Linux iproute2 路由表索引。

默认使用 `2200` + 实例索引。

#### iproute2_rule_index

!!! quote ""

    仅支持 Linux。

Linux iproute2 规则起始索引。

默认使用 `100`。

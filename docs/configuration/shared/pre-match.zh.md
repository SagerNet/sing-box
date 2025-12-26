---
icon: material/new-box
---

# 预匹配

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [bypass](#bypass)

预匹配是在连接建立之前运行的规则匹配。

### 工作原理

当 TUN 收到连接请求时，连接尚未建立，因此无法读取连接数据。在此阶段，sing-box 在预匹配模式下运行路由规则。

由于连接数据不可用，只有不需要连接数据的动作才能执行。当规则匹配到需要已建立连接的动作时，预匹配将在该规则处停止。

### 支持的动作

#### reject

以 TCP RST / ICMP 不可达拒绝。

#### route

将 ICMP 连接路由到指定出站以直接回复。

#### bypass

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux，且需要启用 `auto_redirect`。

在内核层面绕过 sing-box 直接连接。

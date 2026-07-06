---
icon: material/new-box
---

# 预匹配

!!! quote "sing-box 1.14.0 中的更改"

    :material-alert: [route](#route)

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [bypass](#bypass)

预匹配是在连接建立之前运行的规则匹配。

### 工作原理

当 L3 入站（TUN、WireGuard 或 Tailscale）收到连接请求时，连接尚未建立，因此无法读取连接数据。在此阶段，sing-box 在预匹配模式下运行路由规则。

由于连接数据不可用，只有不需要连接数据的动作才能执行。当规则匹配到需要已建立连接的动作时，预匹配将在该规则处停止。

### 支持的动作

#### reject

以 TCP RST / ICMP 不可达拒绝。

详情参阅 [reject](/zh/configuration/route/rule_action/#reject)。

#### route

!!! quote "sing-box 1.14.0 中的更改"

    自 sing-box 1.14.0 起，TCP 和 UDP 连接也可以在 L3 转发；此前仅支持 ICMP 连接。

将连接直接在 L3 转发到指定出站，不经过 L3 到 L4 转换。

支持的目标：

- ICMP 连接：direct 出站和 WireGuard / Tailscale 端点。
- TCP 和 UDP 连接：WireGuard 和 Tailscale 端点。

当没有规则匹配且默认出站为受支持的目标时，L3 转发同样生效；对于出站组，使用当前选中的出站。

FakeIP 目标需要在预匹配中先执行 `resolve` 动作，否则连接将被拒绝。

详情参阅 [route](/zh/configuration/route/rule_action/#route)。

#### bypass

!!! question "自 sing-box 1.13.0 起"

!!! quote ""

    仅支持 Linux，且需要启用 `auto_redirect`。

在内核层面绕过 sing-box 直接连接。

如果未指定 `outbound`，规则仅在来自 auto redirect 的预匹配中匹配，在其他场景中将被跳过。

对于其他所有场景，指定了 `outbound` 的 bypass 行为与 `route` 相同。

详情参阅 [bypass](/zh/configuration/route/rule_action/#bypass)。

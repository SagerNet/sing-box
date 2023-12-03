### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-in",
  ...
  // 监听字段

  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "users": [
    {
      "name": "tobyxdd",
      "password": "goofy_ahh_password"
    }
  ],
  "ignore_client_bandwidth": false,
  "tls": {},
  "masquerade": "",
  "brutal_debug": false
}
```

!!! warning "与官方 Hysteria2 的区别"

    官方程序支持一种名为 **userpass** 的验证方式，
    本质上上是将用户名与密码的组合 `<username>:<password>` 作为实际上的密码，而 sing-box 不提供此别名。
    要将 sing-box 与官方程序一起使用， 您需要填写该组合作为实际密码。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### up_mbps, down_mbps

支持的速率，默认不限制。

与 `ignore_client_bandwidth` 冲突。

#### obfs.type

QUIC 流量混淆器类型，仅可设为 `salamander`。

如果为空则禁用。

#### obfs.password

QUIC 流量混淆器密码.

#### users

Hysteria 用户

#### users.password

认证密码。

#### ignore_client_bandwidth

命令客户端使用 BBR 拥塞控制算法而不是 Hysteria CC。

与 `up_mbps` 和 `down_mbps` 冲突。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

#### masquerade

HTTP3 服务器认证失败时的行为。

| Scheme       | 示例                      | 描述      |
|--------------|-------------------------|---------|
| `file`       | `file:///var/www`       | 作为文件服务器 |
| `http/https` | `http://127.0.0.1:8080` | 作为反向代理  |

如果为空，则返回 404 页。

#### brutal_debug

启用 Hysteria Brutal CC 的调试信息日志记录。

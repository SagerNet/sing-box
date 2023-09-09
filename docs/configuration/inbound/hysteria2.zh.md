### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-in",
  
  ... // 监听字段

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
  "masquerade": "",
  "tls": {}
}
```

!!! warning ""

    默认安装不包含被 Hysteria2 依赖的 QUIC，参阅 [安装](/zh/#_2)。

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

命令客户端使用 BBR 流量控制算法而不是 Hysteria CC。

与 `up_mbps` 和 `down_mbps` 冲突。

#### masquerade

HTTP3 服务器认证失败时的行为。

| Scheme       | 示例                      | 描述      |
|--------------|-------------------------|---------|
| `file`       | `file:///var/www`       | 作为文件服务器 |
| `http/https` | `http://127.0.0.1:8080` | 作为反向代理  |

如果为空，则返回 404 页。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。
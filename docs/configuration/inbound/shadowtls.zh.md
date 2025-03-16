---
icon: material/new-box
---

!!! quote "sing-box 1.12.0 中的更改"

    :material-plus: [wildcard_sni](#wildcard_sni)

### 结构

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // 监听字段

  "version": 3,
  "password": "fuck me till the daylight",
  "users": [
    {
      "name": "sekai",
      "password": "8JCsPssfgS8tiRwiMlhARg=="
    }
  ],
  "handshake": {
    "server": "google.com",
    "server_port": 443,

    ... // 拨号字段
  },
  "handshake_for_server_name": {
    "example.com": {
      "server": "example.com",
      "server_port": 443,
      
      ... // 拨号字段
    }
  },
  "strict_mode": false,
  "wildcard_sni": ""
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### version

ShadowTLS 协议版本。

| 值             | 协议版本                                                                                    |
|---------------|-----------------------------------------------------------------------------------------|
| `1` (default) | [ShadowTLS v1](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v1) |
| `2`           | [ShadowTLS v2](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-en.md#v2) |
| `3`           | [ShadowTLS v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md) |

#### password

ShadowTLS 密码。

仅在 ShadowTLS 协议版本 2 中可用。

#### users

ShadowTLS 用户。

仅在 ShadowTLS 协议版本 3 中可用。

#### handshake

==必填==

握手服务器地址和 [拨号参数](/zh/configuration/shared/dial/)。

#### handshake_for_server_name

==必填==

对于特定服务器名称的握手服务器地址和 [拨号参数](/zh/configuration/shared/dial/)。

仅在 ShadowTLS 协议版本 2/3 中可用。

#### strict_mode

ShadowTLS 严格模式。

仅在 ShadowTLS 协议版本 3 中可用。

#### wildcard_sni

!!! question "自 sing-box 1.12.0 起"

ShadowTLS 通配符 SNI 模式。

可用值：

* `off`：（默认）禁用。
* `authed`：已认证的连接的目标将被重写为 `(servername):443`。
* `all`：所有连接的目标将被重写为 `(servername):443`。

此外，匹配 `handshake_for_server_name` 的连接不受影响。

仅在 ShadowTLS 协议 3 中可用。

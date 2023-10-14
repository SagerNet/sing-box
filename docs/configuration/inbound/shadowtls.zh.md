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
  "strict_mode": false
}
```

!!! error ""

    ShadowTLS 需要和其它基于 TCP 的协议配合使用，参阅 [示例](/zh/examples/shadowtls/)。

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

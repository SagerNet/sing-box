### 结构

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // 监听字段

  "version": 2,
  "password": "fuck me till the daylight",
  "handshake": {
    "server": "google.com",
    "server_port": 443,

    ... // 拨号字段
  }
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

#### password

设置密码。

仅在 ShadowTLS v2 协议中可用。

#### handshake

==必填==

握手服务器地址和 [拨号参数](/zh/configuration/shared/dial/)。

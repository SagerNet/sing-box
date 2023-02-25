### 结构

```json
{
  "type": "vless",
  "tag": "vless-in",

  ... // 监听字段

  "users": [
    {
      "name": "sekai",
      "uuid": "bf000d23-0752-40b4-affe-68f7707a9661"
    }
  ],
  "tls": {},
  "transport": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

VLESS 用户。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

#### transport

V2Ray 传输配置，参阅 [V2Ray 传输层](/zh/configuration/shared/v2ray-transport)。

### 结构

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // 监听字段

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

#### handshake

==必填==

握手服务器地址和 [拨号参数](/zh/configuration/shared/dial/)。

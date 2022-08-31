### 结构

```json
{
  "type": "naive",
  "tag": "naive-in",
  "network": "udp",

  ... // 监听字段

  "users": [
    {
      "username": "sekai",
      "password": "password"
    }
  ],
  "tls": {}
}
```

!!! warning ""

    默认安装不包含 HTTP3 传输层, 参阅 [安装](/zh/#_2)。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### network

监听的网络协议，`tcp` `udp` 之一。

默认所有。

#### users

==必填==

Naive 用户。

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。
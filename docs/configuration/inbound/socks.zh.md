`socks` 入站是一个 socks4, socks4a 和 socks5 服务器.

### 结构

```json
{
  "type": "socks",
  "tag": "socks-in",

  ... // 监听字段

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ]
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

!!! error ""

    对于 UDP 的支持遵照 [RFC 1928](https://datatracker.ietf.org/doc/html/rfc1928)，将使用随机可用的 UDP 端口，而非其他一些流行代理程序中使用的[固定 UDP 端口](https://github.com/v2fly/v2fly-github-io/issues/104)。

### 字段

#### users

SOCKS 用户

如果为空则不需要验证。

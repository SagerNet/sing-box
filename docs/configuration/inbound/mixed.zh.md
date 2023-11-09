`mixed` 入站是一个 socks4, socks4a, socks5 和 http 服务器.

### 结构

```json
{
  "type": "mixed",
  "tag": "mixed-in",

  ... // 监听字段

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ],
  "set_system_proxy": false
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

SOCKS 和 HTTP 用户

如果为空则不需要验证。

#### set_system_proxy

!!! quote ""

    仅支持 Linux、Android、Windows 和 macOS。

!!! warning ""

    要在无特权的 Android 和 iOS 上工作，请改用 tun.platform.http_proxy。

启动时自动设置系统代理，停止时自动清理。
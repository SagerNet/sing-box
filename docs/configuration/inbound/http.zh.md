### 结构

```json
{
  "type": "http",
  "tag": "http-in",

  ... // 监听字段

  "users": [
    {
      "username": "admin",
      "password": "admin"
    }
  ],
  "tls": {},
  "set_system_proxy": false
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### tls

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

#### users

HTTP 用户

如果为空则不需要验证。

#### set_system_proxy

!!! error ""

    仅支持 Linux、Android、Windows 和 macOS。

启动时自动设置系统代理，停止时自动清理。
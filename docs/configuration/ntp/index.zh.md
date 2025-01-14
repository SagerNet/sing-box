# NTP

内建的 NTP 客户端服务。

如果启用，它将为像 TLS/Shadowsocks/VMess 这样的协议提供时间，这对于无法进行时间同步的环境很有用。

### 结构

```json
{
  "ntp": {
    "enabled": false,
    "server": "time.apple.com",
    "server_port": 123,
    "interval": "30m",
    "write_to_system": false

    ... // Dial Fields
  }
}

```

### 字段

#### enabled

启用 NTP 服务。

#### server

==必填==

NTP 服务器地址。

#### server_port

NTP 服务器端口。

默认使用 123。

#### interval

时间同步间隔。

默认使用 30 分钟。

#### write_to_system

写入系统时间。

默认不写入。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

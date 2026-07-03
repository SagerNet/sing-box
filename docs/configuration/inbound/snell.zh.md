---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "type": "snell",
  "tag": "snell-in",

  ... // 监听字段

  "version": 5,
  "psk": "password",
  "users": [
    {
      "name": "sekai",
      "userkey": "user-password"
    }
  ],
  "obfs_mode": ""
}
```

### 版本 6 结构

```json
{
  "type": "snell",
  "tag": "snell-in",

  ... // 监听字段

  "version": 6,
  "psk": "password",
  "users": [
    {
      "name": "sekai",
      "userkey": "user-password"
    }
  ],
  "mode": ""
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### version

==必填==

Snell 协议版本，`5` `6` 之一。

版本 `5` 支持 HTTP 混淆（`obfs_mode`）；版本 `6` 以流量整形（`mode`）取而代之，并要求
`psk` 长度为 12 到 255 字节。

!!! note

    由于我们有意不支持 Snell v5 的 QUIC 代理模式，v5 的线路协议实际上与 v4 没有区别，
    因此不提供独立的 v4 服务器和 v5 客户端。

#### psk

==必填==

预共享密钥。

#### users

Snell 用户。

设置后，服务器运行于多用户模式：每一项包含 `name`（可选，用于日志）和 `userkey`
（用户密钥）。顶层的 `psk` 仍作为服务器密钥。

#### obfs_mode

==仅版本 5==

HTTP 混淆模式，`none` `http` 之一。

默认为 `none`。

#### mode

==仅版本 6==

流量整形模式，`default` `unshaped` `unsafe-raw` 之一。

默认为 `default`。

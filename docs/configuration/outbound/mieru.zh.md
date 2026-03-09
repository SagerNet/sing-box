---
icon: material/new-box
---

### 结构

```json
{
  "type": "mieru",
  "tag": "mieru-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "server_ports": [
    "9000-9010",
    "9020-9030"
  ],
  "transport": "TCP",
  "username": "asdf",
  "password": "hjkl",
  "multiplexing": "MULTIPLEXING_LOW",

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

服务器端口。

必须填写 `server_port` 和 `server_ports` 中至少一项。

#### server_ports

服务器端口范围列表。

必须填写 `server_port` 和 `server_ports` 中至少一项。

#### transport

==必填==

通信协议。仅可设为 `TCP`。

#### username

==必填==

mieru 用户名。

#### password

==必填==

mieru 密码。

#### multiplexing

多路复用设置。可以设为 `MULTIPLEXING_OFF`，`MULTIPLEXING_LOW`，`MULTIPLEXING_MIDDLE`，`MULTIPLEXING_HIGH`。其中 `MULTIPLEXING_OFF` 会关闭多路复用功能。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

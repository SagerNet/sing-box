### 服务器要求

`sing-box` :)

### 结构

```json
{
  "enabled": true,
  "protocol": "smux",
  "max_connections": 4,
  "min_streams": 4,
  "max_streams": 0
}
```

### 字段

#### enabled

启用多路复用。

#### protocol

多路复用协议

| 协议    | 描述                                 |
|-------|------------------------------------|
| smux  | https://github.com/xtaci/smux      |
| yamux | https://github.com/hashicorp/yamux |
| h2mux | https://golang.org/x/net/http2     |

默认使用 h2mux。

#### max_connections

最大连接数量。

与 `max_streams` 冲突。

#### min_streams

在打开新连接之前，连接中的最小多路复用流数量。

与 `max_streams` 冲突。

#### max_streams

在打开新连接之前，连接中的最大多路复用流数量。

与 `max_connections` 和 `min_streams` 冲突。

#### padding

!!! info

    需要 sing-box 服务器版本 1.3-beta9 或更高。

启用填充。


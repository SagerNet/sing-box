V2Ray Transport 是 v2ray 发明的一组私有协议，并污染了其他协议的名称，如 clash 中的 `trojan-grpc`。

### 结构

```json
{
  "type": ""
}
```

可用的传输协议：

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* XHTTP

!!! warning "与 v2ray-core 的区别"

    * 没有 TCP 传输层, 纯 HTTP 已合并到 HTTP 传输层。
    * 没有 mKCP 传输层。
    * 没有 DomainSocket 传输层。

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### HTTP

```json
{
  "type": "http",
  "host": [],
  "path": "",
  "method": "",
  "headers": {},
  "idle_timeout": "15s",
  "ping_timeout": "15s"
}
```

!!! warning "与 v2ray-core 的区别"

    不强制执行 TLS。如果未配置 TLS，将使用纯 HTTP 1.1。

#### host

主机域名列表。

如果设置，客户端将随机选择，服务器将验证。

#### path

!!! warning

    V2Ray 文档称服务端和客户端的路径必须一致，但实际代码允许客户端向路径添加任何后缀。
    sing-box 使用与 V2Ray 相同的行为，但请注意，该行为在 `WebSocket` 和 `HTTPUpgrade` 传输层中不存在。

HTTP 请求路径

服务器将验证。

#### method

HTTP 请求方法

如果设置，服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

#### idle_timeout

在 HTTP2 服务器中：

指定闲置客户端应在多长时间内使用 GOAWAY 帧关闭。PING 帧不被视为活动。

在 HTTP2 客户端中：

如果连接上没有收到任何帧，指定一段时间后将使用 PING 帧执行健康检查。需要注意的是，PING 响应被视为已接收的帧，因此如果连接上没有其他流量，则健康检查将在每个间隔执行一次。如果值为零，则不会执行健康检查。

默认使用零。

#### ping_timeout

在 HTTP2 客户端中：

指定发送 PING 帧后，在指定的超时时间内必须接收到响应。如果在指定的超时时间内没有收到 PING 帧的响应，则连接将关闭。默认超时持续时间为 15 秒。

### WebSocket

```json
{
  "type": "ws",
  "path": "",
  "headers": {},
  "max_early_data": 0,
  "early_data_header_name": ""
}
```

#### path

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

### XHTTP

```json
{
  "type": "xhttp",
  "host": "",
  "path": "",
  "mode": "auto",
  "x_padding_bytes": { "from": 100, "to": 1000 },
  "sc_max_each_post_bytes": { "from": 1000000, "to": 1000000 }
}
```

XHTTP 与 Xray 的 XHTTP 传输层兼容。它将一个逻辑双向连接拆为流式下行，
以及流式或分包式上行。`stream-one` 在一个请求中传输双向数据；`stream-up`
使用一个下行请求和一个流式上行请求；`packet-up` 使用一个下行请求和按序上行
分包。`auto` 当前选择 `packet-up`；当使用 REALITY 和 `download_settings` 时，
会选择 `stream-up`。

本版本支持 Xray 的 session/seq placement、请求 padding，以及 body/header/cookie
上行载荷。支持 HTTP/1.1、HTTP/2 和 h2c。`download_settings` 可以将下行请求
发往独立的 XHTTP 目标，而上行请求仍使用主目标。两个目标必须到达同一个 XHTTP
服务端会话（例如同一后端的不同 CDN 入口）；不能与 `stream-one` 一同使用。

```json
{
  "type": "xhttp",
  "path": "/upload",
  "download_settings": {
    "server": "download.example.com",
    "server_port": 443,
    "tls": {
      "enabled": true,
      "server_name": "download.example.com"
    },
    "path": "/download",
    "mode": "packet-up"
  }
}
```

`download_settings` 内嵌另一套 XHTTP 配置，并附带其 `server`、`server_port`
和可选的 `tls` 设置。

使用 `with_quic` 构建 sing-box，并将外层入站或出站 TLS 的 ALPN 精确设为 `h3`
时可使用 HTTP/3。该路径使用标准 TLS，暂不支持 uTLS 或 REALITY。可选的 `quic` 对象支持常用
QUIC 参数：`idle_timeout`、`keep_alive_period`、`stream_receive_window`、
`connection_receive_window`、`max_concurrent_streams`、`initial_packet_size`
和 `disable_path_mtu_discovery`。

```json
{
  "type": "xhttp",
  "path": "/xhttp/",
  "quic": {
    "keep_alive_period": "15s",
    "max_concurrent_streams": 32
  }
}
```

#### max_early_data

请求中允许的最大有效负载大小。默认启用。

#### early_data_header_name

默认情况下，早期数据在路径而不是标头中发送。

要与 Xray-core 兼容，请将其设置为 `Sec-WebSocket-Protocol`。

它需要与服务器保持一致。

### QUIC

```json
{
  "type": "quic"
}
```

!!! warning "与 v2ray-core 的区别"

    没有额外的加密支持：
    它基本上是重复加密。 并且 Xray-core 在这里与 v2ray-core 不兼容。

### gRPC

!!! note ""

    默认安装不包含标准 gRPC (兼容性好，但性能较差), 参阅 [安装](/zh/installation/build-from-source/#构建标记)。

```json
{
  "type": "grpc",
  "service_name": "TunService",
  "idle_timeout": "15s",
  "ping_timeout": "15s",
  "permit_without_stream": false
}
```

#### service_name

gRPC 服务名称。

#### idle_timeout

在标准 gRPC 服务器/客户端：

如果传输在此时间段后没有看到任何活动，它会向客户端发送 ping 请求以检查连接是否仍然活动。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### ping_timeout

在标准 gRPC 服务器/客户端：

经过一段时间之后，客户端将执行 keepalive 检查并等待活动。如果没有检测到任何活动，则会关闭连接。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### permit_without_stream

在标准 gRPC 客户端：

如果启用，客户端传输即使没有活动连接也会发送 keepalive ping。如果禁用，则在没有活动连接时，将忽略 `idle_timeout` 和 `ping_timeout`，并且不会发送 keepalive ping。

默认禁用。

### HTTPUpgrade

```json
{
  "type": "httpupgrade",
  "host": "",
  "path": "",
  "headers": {}
}
```

#### host

主机域名。

服务器将验证。

#### path

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头。

如果设置，服务器将写入响应。

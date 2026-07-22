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

#### max_early_data

WebSocket 握手请求中允许携带的最大有效负载。非零时启用。

#### early_data_header_name

默认将早期数据放在路径中而非标头中。为兼容 Xray-core，可设为
`Sec-WebSocket-Protocol`；客户端和服务端必须保持一致。

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

XHTTP 与 Xray 的 XHTTP 传输层兼容。它把一个逻辑双向连接拆分为流式下行，以及
流式或分包式上行。支持 HTTP/1.1、HTTP/2 和 h2c。

除非另有说明，客户端与服务端必须使用相同的值。范围值为包含端点的随机区间
`{ "from": n, "to": n }`。全零范围会使用该字段的协议默认值；非零范围必须满足
`from > 0` 且 `to >= from`。

#### type

必须为 `xhttp`。

#### 范围对象

每个 XHTTP 范围对象都有以下字段：

* `from`：包含在内的下界。
* `to`：包含在内的上界。为 `0` 时，整个范围使用文档中指定的协议默认值。

#### host

期望的 HTTP `Host`。客户端以此作为请求 Host；服务端配置为非空时会拒绝其他
Host。不要在 `headers` 中设置 `Host`。

#### path

请求基础路径。空值会变为 `/`，缺少前导 `/` 时会自动补上。可以包含查询字符串，
它会保留在每个请求中。若 session 或 seq 使用 `path` placement，服务端会接受在
该基础路径后追加的会话和序号路径段。

#### mode

上行模式：

* `stream-one`：一个全双工请求同时承载双向数据。
* `stream-up`：一个 GET 下行请求加一个流式上行请求。
* `packet-up`：一个 GET 下行请求加多个按序、独立的上行请求。
* `auto`（默认）：选择 `packet-up`；使用 REALITY 时选择 `stream-one`，若同时
  配置 `download_settings` 则选择 `stream-up`。

`stream-one` 不能使用 `download_settings`。`uplink_http_method` 为 `GET` 时仅可
用于 `packet-up`（或在非 REALITY 场景下会解析为它的 `auto`）。

#### headers

额外的请求标头，禁止包含 `Host`。若前置服务器会校验这些标头，客户端和服务端
应使用相同的值。

#### x_padding_bytes

请求和响应 padding 的闭区间字节数，默认 `100`–`1000`。此实现需要 padding；
全零范围表示使用默认值，而不是关闭它。

#### x_padding_obfs_mode

启用以下可配置的 padding 形态，默认 `false`。为 `false` 时，线路格式固定为请求
`Referer: ...?x_padding=XXXXX` 与响应 `X-Padding` 标头；`x_padding_key`、
`x_padding_header`、`x_padding_placement` 和 `x_padding_method` 将被忽略。

#### x_padding_key

启用 padding 混淆模式时所用的 query 或 cookie 键名，默认 `x_padding`。

#### x_padding_header

placement 为 `header` 或 `queryInHeader` 时的标头名称，默认 `X-Padding`。

#### x_padding_placement

启用 padding 混淆模式时的存放位置：`cookie`、`header`、`query` 或
`queryInHeader`（默认）。`queryInHeader` 会把带 padding 查询参数的 URL 序列化到
指定标头中。

#### x_padding_method

Padding 编码：`repeat-x`（默认）发送重复的 `X`；`tokenish` 生成按 HPACK Huffman
长度控制、类似浏览器的 Base62 token。

#### uplink_http_method

流式和分包式上行的 HTTP 方法，会被转换为大写，默认 `POST`。`GET` 的限制见
`mode`。

#### session_id_placement

会话 ID 的传递位置：`path`（默认）、`query`、`header` 或 `cookie`。它把同一逻辑
连接的下行请求和上行请求关联起来。

#### session_id_key

session ID 使用 query、header 或 cookie 时的键名；`path` 时无效。默认值为：
`query`/`cookie` 使用 `x_session`，`header` 使用 `X-Session`。

#### session_id_table

生成 session ID 的字符表。可使用自定义 ASCII 字符串，或 `ALPHABET`、`Alphabet`、
`BASE36`、`Base62`、`HEX`、`alphabet`、`base36`、`hex`、`number`。需同时设置
`session_id_length` 才会启用；否则使用 UUID 形态的 session ID。

#### session_id_length

使用 `session_id_table` 生成 ID 时的随机长度闭区间。未设置字符表时无效。

#### seq_placement

`packet-up` 递增包序号的传递位置：`path`（默认）、`query`、`header` 或 `cookie`。

#### seq_key

包序号使用 query、header 或 cookie 时的键名；`path` 时无效。默认值为：
`query`/`cookie` 使用 `x_seq`，`header` 使用 `X-Seq`。

#### uplink_data_placement

`packet-up` 有效载荷的位置：`body`（默认）、`header`、`cookie` 或 `auto`。
`header` 与 `cookie` 会将数据 Base64URL 编码并分块；`auto` 让服务端合并 header、
cookie 和 body 数据。该字段不用于流式上行。

#### uplink_data_key

header 或 cookie 分包有效载荷的基础键名。header 分块为 `<key>-0`、`<key>-1`…；
cookie 分块为 `<key>_0`、`<key>_1`…。默认值为：`header`/`auto` 使用 `X-Data`，
`cookie` 使用 `x_data`；`body` 时无效。

#### uplink_chunk_size

header 或 cookie 分包有效载荷的编码后分块大小闭区间。小于 64 的值会提升为 64。
默认值：`header` 为 `3000`–`4000`，`cookie` 为 `2048`–`3072`，其他情况使用
`sc_max_each_post_bytes`。

#### no_grpc_header

不在流式上行请求中添加 `Content-Type: application/grpc`，默认 `false`。

#### no_sse_header

不在服务端下行响应中添加 `Content-Type: text/event-stream`，默认 `false`。

#### sc_max_each_post_bytes

一个 `packet-up` 请求体的随机最大字节数。服务端会拒绝更大的上行，默认
`1000000` 字节；客户端与服务端应使用兼容的值。

#### sc_min_posts_interval_ms

`packet-up` 请求之间的最小随机间隔，单位毫秒，默认 `30`–`30`。这是客户端的
发送节流控制。

#### sc_max_buffered_posts

服务端在等待下一个序号时可缓存的乱序 `packet-up` 请求数量，默认 `30`，必须至少
为 `1`。

#### sc_stream_up_server_secs

当 stream-up 请求带有 `Referer` 标头或启用 padding 混淆时，服务端向 stream-up
响应写入 keepalive padding 的随机间隔，单位秒，默认 `20`–`80`。

#### server_max_header_bytes

XHTTP 服务端接受的 HTTP 请求标头最大长度，默认 `8192` 字节；负数无效。

#### xmux

客户端 HTTP transport 复用控制。每个范围在创建 HTTP client 时随机选择；全零
范围表示关闭对应限制。`max_connections` 与 `max_concurrency` 不能同时设置。

* `max_concurrency`：单个 HTTP client 可共享的逻辑 XHTTP 连接数上限。
* `max_connections`：复用 HTTP client 池的数量上限。
* `c_max_reuse_times`：一个 HTTP client 允许再次分配给逻辑连接的次数。
* `h_max_request_times`：一个 HTTP client 可发送的 HTTP 请求总数。
* `h_max_reusable_secs`：HTTP client 退役前的最长可复用时间（秒）。
* `h_keep_alive_period`：HTTP client keepalive 周期（秒）；`0` 关闭，负数无效。

#### download_settings

可选的独立 XHTTP 下行 GET 目标；上行仍使用外层目标。它包含 `server`、
`server_port`、可选的 `tls`，以及本节中除 `download_settings` 本身外的全部 XHTTP
字段。`server` 和 `server_port` 必填，不支持嵌套 `download_settings`。两个目标必须
到达同一个服务端 XHTTP session。

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

#### quic

HTTP/3 设置。须使用 `with_quic` 构建 sing-box，并将外层入站或出站 TLS 的 ALPN
精确设为 `h3`。该路径要求标准 TLS，不能使用 uTLS 或 REALITY。对象采用常规 QUIC
字段：`idle_timeout`、`keep_alive_period`、`stream_receive_window`、
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

V2Ray Transport is a set of private protocols invented by v2ray, and has contaminated the names of other protocols, such
as `trojan-grpc` in clash.

### Structure

```json
{
  "type": ""
}
```

Available transports:

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* XHTTP

!!! warning "Difference from v2ray-core"

    * No TCP transport, plain HTTP is merged into the HTTP transport.
    * No mKCP transport.
    * No DomainSocket transport.

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

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

!!! warning "Difference from v2ray-core"

    TLS is not enforced. If TLS is not configured, plain HTTP 1.1 is used.

#### host

List of host domain.

The client will choose randomly and the server will verify if not empty.

#### path

!!! warning

    V2Ray's documentation says that the path between the server and the client must be consistent, 
    but the actual code allows the client to add any suffix to the path.
    sing-box uses the same behavior as V2Ray, but note that the behavior does not exist in `WebSocket` and `HTTPUpgrade` transport.

Path of HTTP request.

The server will verify.

#### method

Method of HTTP request.

The server will verify if not empty.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### idle_timeout

In HTTP2 server:

Specifies the time until idle clients should be closed with a GOAWAY frame. PING frames are not considered as activity.

In HTTP2 client:

Specifies the period of time after which a health check will be performed using a ping frame if no frames have been
received on the connection.Please note that a ping response is considered a received frame, so if there is no other
traffic on the connection, the health check will be executed every interval. If the value is zero, no health check will
be performed.

Zero is used by default.

#### ping_timeout

In HTTP2 client:

Specifies the timeout duration after sending a PING frame, within which a response must be received.
If a response to the PING frame is not received within the specified timeout duration, the connection will be closed.
The default timeout duration is 15 seconds.

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

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### max_early_data

Allowed payload size in the WebSocket handshake request. Enabled if not zero.

#### early_data_header_name

By default, early data is sent in the path rather than a header. Set it to
`Sec-WebSocket-Protocol` for Xray-core compatibility. It must match on both
ends.

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

XHTTP is compatible with Xray's XHTTP transport. It carries one logical
bidirectional connection as a streaming download plus either a streaming or
packetized upload. HTTP/1.1, HTTP/2 and h2c are supported.

Unless noted otherwise, client and server must use the same values. A range is
an inclusive `{ "from": n, "to": n }` random range. A zero range selects that
field's protocol default; non-zero ranges must have `from > 0` and `to >= from`.

#### type

Must be `xhttp`.

#### Range objects

Every XHTTP range object has these fields:

* `from`: inclusive lower bound.
* `to`: inclusive upper bound. When it is `0`, the whole range uses the
  documented protocol default.

#### host

The expected HTTP `Host`. The client uses it as the request host; a non-empty
value makes the server reject other hosts. Do not put `Host` in `headers`.

#### path

Base request path. An empty value becomes `/`; a leading `/` is added when
missing. The value may include a query string, which is kept on every request.
The server accepts this base path followed by the session and sequence path
components when those placements use `path`.

#### mode

The upload mode:

* `stream-one`: one full-duplex request carries both directions.
* `stream-up`: a GET download request plus a streaming upload request.
* `packet-up`: a GET download request plus ordered, independent upload
  requests.
* `auto` (default): `packet-up`; with REALITY it selects `stream-one`, or
  `stream-up` when `download_settings` is configured.

`download_settings` cannot be used with `stream-one`. An
`uplink_http_method` of `GET` is only valid with `packet-up` (or `auto`, which
resolves to it outside REALITY).

#### headers

Additional request headers. `Host` is forbidden. The same values should be
configured on both ends when the fronting server validates them.

#### x_padding_bytes

Inclusive byte range for request and response padding. Default: `100` to
`1000`. Padding is required by this implementation; a zero range means the
default rather than disabled.

#### x_padding_obfs_mode

Enables the configurable padding shape below. Default: `false`. When false,
the wire format is fixed to `Referer: ...?x_padding=XXXXX` for requests and an
`X-Padding` response header; `x_padding_key`, `x_padding_header`,
`x_padding_placement`, and `x_padding_method` are ignored.

#### x_padding_key

Query or cookie key for padding when `x_padding_obfs_mode` is enabled. Default:
`x_padding`.

#### x_padding_header

Header name for padding when its placement is `header` or `queryInHeader`.
Default: `X-Padding`.

#### x_padding_placement

Where to put padding when obfuscation mode is enabled: `cookie`, `header`,
`query`, or `queryInHeader` (default). `queryInHeader` serializes a URL with
the padding query parameter into the configured header.

#### x_padding_method

Padding encoding: `repeat-x` (default) sends repeated `X` characters;
`tokenish` generates a browser-like Base62 token sized by HPACK Huffman length.

#### uplink_http_method

HTTP method for stream uploads and packet uploads. It is normalized to upper
case and defaults to `POST`. `GET` is restricted as described under `mode`.

#### session_id_placement

Where the session ID is sent: `path` (default), `query`, `header`, or `cookie`.
It identifies the download and upload requests that form one connection.

#### session_id_key

Query, header, or cookie key for the session ID. It is unused with `path`.
Defaults are `x_session` for `query`/`cookie` and `X-Session` for `header`.

#### session_id_table

Character table used to generate session IDs. It can be a custom ASCII string
or one of `ALPHABET`, `Alphabet`, `BASE36`, `Base62`, `HEX`, `alphabet`,
`base36`, `hex`, or `number`. Set `session_id_length` as well to activate it;
otherwise XHTTP uses a UUID-shaped session ID.

#### session_id_length

Inclusive random length for IDs generated from `session_id_table`. It has no
effect without a non-empty table.

#### seq_placement

Where `packet-up` sends its monotonically increasing packet sequence: `path`
(default), `query`, `header`, or `cookie`.

#### seq_key

Query, header, or cookie key for the packet sequence. It is unused with
`path`. Defaults are `x_seq` for `query`/`cookie` and `X-Seq` for `header`.

#### uplink_data_placement

Where `packet-up` carries packet payloads: `body` (default), `header`,
`cookie`, or `auto`. `header` and `cookie` Base64URL-encode and split the
payload; `auto` lets the server combine header, cookie, and body data. This
setting is not used by stream uploads.

#### uplink_data_key

Base key for header or cookie packet payloads. Header chunks use
`<key>-0`, `<key>-1`, …; cookie chunks use `<key>_0`, `<key>_1`, …. Defaults:
`X-Data` for `header`/`auto` and `x_data` for `cookie`. It is unused for `body`.

#### uplink_chunk_size

Inclusive encoded chunk-size range for header or cookie packet payloads.
Values below 64 are raised to 64. Defaults: `3000`–`4000` for `header`,
`2048`–`3072` for `cookie`, and `sc_max_each_post_bytes` otherwise.

#### no_grpc_header

Do not add `Content-Type: application/grpc` to streaming upload requests.
Default: `false`.

#### no_sse_header

Do not add `Content-Type: text/event-stream` to server download responses.
Default: `false`.

#### sc_max_each_post_bytes

Inclusive maximum size of one `packet-up` request body. The server rejects
larger uploads. Default: `1000000` bytes. Keep client and server values
compatible.

#### sc_min_posts_interval_ms

Inclusive minimum delay, in milliseconds, between `packet-up` requests.
Default: `30`–`30`. It is a client-side pacing control.

#### sc_max_buffered_posts

Maximum number of out-of-order `packet-up` requests the server buffers while
waiting for the next sequence. Default: `30`; it must be at least `1`.

#### sc_stream_up_server_secs

Inclusive interval, in seconds, for server keepalive padding on a stream-up
response when the request has a `Referer` header or padding obfuscation is
enabled. Default: `20`–`80`.

#### server_max_header_bytes

Maximum HTTP request-header size accepted by the XHTTP server. Default: `8192`
bytes. A negative value is invalid.

#### xmux

Controls client-side reuse of HTTP transports. Each range is selected when an
HTTP client is created; zero disables that limit. `max_connections` and
`max_concurrency` are mutually exclusive.

* `max_concurrency`: maximum logical XHTTP connections sharing one HTTP
  client.
* `max_connections`: maximum reusable HTTP clients in the pool.
* `c_max_reuse_times`: number of subsequent logical-connection reuses allowed
  for an HTTP client.
* `h_max_request_times`: total HTTP requests allowed for an HTTP client.
* `h_max_reusable_secs`: lifetime in seconds before an HTTP client is retired.
* `h_keep_alive_period`: HTTP client keepalive period in seconds; `0` disables
  it. Negative values are invalid.

#### download_settings

Optional independent XHTTP target for the download GET request. Uploads still
use the enclosing target. It contains `server`, `server_port`, optional `tls`,
and every XHTTP field in this section except another `download_settings`.
`server` and `server_port` are required; nested `download_settings` is not
supported. Both targets must reach the same server-side XHTTP session.

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

`download_settings` embeds another XHTTP configuration, plus its `server`,
`server_port`, and optional `tls` settings.

#### quic

HTTP/3 settings. Build sing-box with `with_quic` and set the enclosing inbound
or outbound TLS ALPN to exactly `h3`. This path requires standard TLS; uTLS and
REALITY are unavailable. The object uses the normal QUIC fields:
`idle_timeout`, `keep_alive_period`, `stream_receive_window`,
`connection_receive_window`, `max_concurrent_streams`, `initial_packet_size`,
and `disable_path_mtu_discovery`.

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

!!! warning "Difference from v2ray-core"

    No additional encryption support:
    It's basically duplicate encryption. And Xray-core is not compatible with v2ray-core in here.

### gRPC

!!! note ""

    standard gRPC has good compatibility but poor performance and is not included by default, see [Installation](/installation/build-from-source/#build-tags).

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

Service name of gRPC.

#### idle_timeout

In standard gRPC server/client:

If the transport doesn't see any activity after a duration of this time,
it pings the client to check if the connection is still active.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### ping_timeout

In standard gRPC server/client:

The timeout that after performing a keepalive check, the client will wait for activity.
If no activity is detected, the connection will be closed.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### permit_without_stream

In standard gRPC client:

If enabled, the client transport sends keepalive pings even with no active connections.
If disabled, when there are no active connections, `idle_timeout` and `ping_timeout` will be ignored and no keepalive
pings will be sent.

Disabled by default.

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

Host domain.

The server will verify if not empty.

#### path

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

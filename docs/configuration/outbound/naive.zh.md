---
icon: material/new-box
---

!!! question "自 sing-box 1.13.0 起"

### 结构

```json
{
  "type": "naive",
  "tag": "naive-out",

  "server": "127.0.0.1",
  "server_port": 443,
  "username": "sekai",
  "password": "password",
  "insecure_concurrency": 0,
  "extra_headers": {},
  "udp_over_tcp": false | {},
  "tls": {},

  ... // 拨号字段
}
```

!!! warning ""

    NaiveProxy 出站仅在 Apple 平台、Android、Windows 和部分架构的 Linux 上可用，参阅 [从源代码构建](/zh/installation/build-from-source/#with_naive_outbound)。

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### username

认证用户名。

#### password

认证密码。

#### insecure_concurrency

并发隧道连接数。多连接使隧道更容易被流量分析检测，违背 NaiveProxy 抵抗流量分析的设计目的。

#### extra_headers

HTTP 请求中发送的额外头部。

#### udp_over_tcp

UDP over TCP 配置。

参阅 [UDP Over TCP](/zh/configuration/shared/udp-over-tcp/)。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#outbound)。

只有 `server_name`、`certificate`、`certificate_path` 和 `certificate_public_key_sha256` 是被支持的。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

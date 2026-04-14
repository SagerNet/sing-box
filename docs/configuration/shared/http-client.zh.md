---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

字符串或对象。

当为字符串时，为顶层 `http_clients` 中定义的共享 [HTTP 客户端](/zh/configuration/shared/http-client/) 的标签。

当为对象时：

```json
{
  "engine": "",
  "version": 0,
  "disable_version_fallback": false,
  "headers": {},

  ... // HTTP2 字段

  "tls": {},

  ... // 拨号字段
}
```

### 字段

#### engine

要使用的 HTTP 引擎。

可用值：

* `go`（默认）
* `apple`

`apple` 使用 NSURLSession，仅在 Apple 平台可用。

!!! warning ""

    仅供实验用途：由于 CGO 和 Network.framework 占用的内存都很多，
    不应在 iOS 和 tvOS 的热路径中使用。

支持的字段：

* `headers`
* `tls.server_name`（必须与请求主机匹配）
* `tls.insecure`
* `tls.min_version` / `tls.max_version`
* `tls.certificate` / `tls.certificate_path`
* `tls.certificate_public_key_sha256`
* 拨号字段

不支持的字段：

* `version`
* `disable_version_fallback`
* HTTP2 字段
* QUIC 字段
* `tls.engine`
* `tls.alpn`
* `tls.disable_sni`
* `tls.cipher_suites`
* `tls.curve_preferences`
* `tls.client_certificate` / `tls.client_certificate_path` / `tls.client_key` / `tls.client_key_path`
* `tls.fragment` / `tls.record_fragment`
* `tls.kernel_tx` / `tls.kernel_rx`
* `tls.ech`
* `tls.utls`
* `tls.reality`

#### version

HTTP 版本。

可用值：`1`、`2`、`3`。

默认使用 `2`。

当为 `3` 时，[HTTP2 字段](#http2-字段) 替换为 [QUIC 字段](#quic-字段)。

#### disable_version_fallback

禁用自动回退到更低的 HTTP 版本。

#### headers

自定义 HTTP 标头。

`Host` 标头用作请求主机。

### HTTP2 字段

当 `version` 为 `2`（默认）时。

参阅 [HTTP2 字段](/zh/configuration/shared/http2/) 了解详情。

### QUIC 字段

当 `version` 为 `3` 时。

参阅 [QUIC 字段](/zh/configuration/shared/quic/) 了解详情。

### TLS 字段

参阅 [TLS](/zh/configuration/shared/tls/#出站) 了解详情。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。

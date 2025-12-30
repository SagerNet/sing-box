### 结构

```json
{
  "type": "sudoku",
  "tag": "sudoku-in",

  ... // 监听字段

  "key": "test_key",
  "aead": "chacha20-poly1305",
  "padding_min": 10,
  "padding_max": 30,
  "ascii": "prefer_ascii",
  "custom_table": "",
  "custom_tables": [],
  "enable_pure_downlink": true,
  "handshake_timeout": 5,
  "disable_http_mask": false,
  "http_mask_mode": "legacy"
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### key

==必填==

Sudoku 密钥。

可以是共享密钥字符串（两端填写相同值），也可以是 Ed25519 密钥。

如需使用 Ed25519 密钥对模式，可通过 `sing-box generate sudoku-keypair` 生成，然后将入站 `key` 设置为 `PublicKey`，出站 `key` 设置为 `PrivateKey`。

#### aead

AEAD 方法。

可选值：

* `aes-128-gcm`
* `chacha20-poly1305`
* `none`

#### padding_min

填充最小比例（0-100）。

#### padding_max

填充最大比例（0-100）。

#### ascii

表模式。

可选值：

* `prefer_ascii`
* `prefer_entropy`

#### custom_table

entropy 模式的自定义表 pattern。

必须包含 8 个符号，且恰好 2 个 `x`、2 个 `p`、4 个 `v`，例如 `xpxvvpvv`。

当 `ascii` 为 `prefer_ascii` 时会被忽略。

#### custom_tables

自定义表 patterns（轮换）。

启用后，客户端与服务端必须提供相同的 pattern 列表。

#### enable_pure_downlink

启用 pure downlink 模式。

设为 `false` 会使用 packed downlink 模式（要求使用 AEAD，不能设置 `aead: none`）。

#### handshake_timeout

握手超时时间（秒）。

#### disable_http_mask

禁用所有 HTTP 伪装层。

#### http_mask_mode

HTTP 伪装模式。

可选值：

* `legacy`（写入伪造 HTTP 头，无法通过 CDN）
* `stream`（真实 HTTP 流式隧道，可通过 CDN）
* `poll`（真实 HTTP 轮询隧道）
* `auto`（同时接受 stream 与 poll）

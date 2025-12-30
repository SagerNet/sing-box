### 结构

```json
{
  "type": "sudoku",
  "tag": "sudoku-out",

  "server": "127.0.0.1",
  "server_port": 1080,
  "key": "test_key",
  "aead": "chacha20-poly1305",
  "padding_min": 10,
  "padding_max": 30,
  "ascii": "prefer_ascii",
  "custom_table": "",
  "custom_tables": [],
  "enable_pure_downlink": true,
  "disable_http_mask": false,
  "http_mask_mode": "legacy",
  "http_mask_tls": false,
  "http_mask_host": "",
  "http_mask_strategy": "random",

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

==必填==

服务器端口。

#### key

==必填==

Sudoku 密钥。

可以是共享密钥字符串（两端填写相同值），也可以是 Ed25519 密钥。

如需使用 Ed25519 密钥对模式，可通过 `sing-box generate sudoku-keypair` 生成，然后将出站 `key` 设置为 `PrivateKey`，入站 `key` 设置为 `PublicKey`。

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

#### disable_http_mask

禁用所有 HTTP 伪装层。

#### http_mask_mode

HTTP 伪装模式。

可选值：

* `legacy`（写入伪造 HTTP 头，无法通过 CDN）
* `stream`（真实 HTTP 流式隧道，可通过 CDN）
* `poll`（真实 HTTP 轮询隧道）
* `auto`（先尝试 stream，失败后回退到 poll）

#### http_mask_tls

为 `http_mask_mode` 为 `stream`/`poll`/`auto` 时启用 HTTPS。

#### http_mask_host

为 `http_mask_mode` 为 `stream`/`poll`/`auto` 时覆盖 HTTP Host 头 / SNI Host。

#### http_mask_strategy

`http_mask_mode` 为 `legacy` 时使用的 HTTP 头模板策略。

可选值：

* `random`
* `post`
* `websocket`

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

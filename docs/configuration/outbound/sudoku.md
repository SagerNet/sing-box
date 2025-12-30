### Structure

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

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

The server address.

#### server_port

==Required==

The server port.

#### key

==Required==

Sudoku key.

It can be a shared secret string (same value on both sides) or an Ed25519 key.

To use Ed25519 key pair mode, generate one with `sing-box generate sudoku-keypair`, then set outbound `key` to `PrivateKey` and inbound `key` to `PublicKey`.

#### aead

AEAD method.

Available values:

* `aes-128-gcm`
* `chacha20-poly1305`
* `none`

#### padding_min

Padding min rate (0-100).

#### padding_max

Padding max rate (0-100).

#### ascii

Table mode.

Available values:

* `prefer_ascii`
* `prefer_entropy`

#### custom_table

Custom table pattern for entropy mode.

It must contain 8 symbols, exactly 2 `x`, 2 `p` and 4 `v`, e.g. `xpxvvpvv`.

Ignored when `ascii` is `prefer_ascii`.

#### custom_tables

Custom table patterns (rotation).

When enabled, both client and server must provide the same pattern list.

#### enable_pure_downlink

Enable pure downlink mode.

Set to `false` to use packed downlink mode (requires AEAD, `aead: none` is not allowed).

#### disable_http_mask

Disable all HTTP masking layers.

#### http_mask_mode

HTTP masking mode.

Available values:

* `legacy` (write a fake HTTP header, not CDN compatible)
* `stream` (real HTTP streaming tunnel, CDN compatible)
* `poll` (real HTTP polling tunnel)
* `auto` (try stream then fall back to poll)

#### http_mask_tls

Enable HTTPS for `http_mask_mode` `stream`/`poll`/`auto`.

#### http_mask_host

Override HTTP Host header / SNI host for `http_mask_mode` `stream`/`poll`/`auto`.

#### http_mask_strategy

HTTP header template strategy for `http_mask_mode` `legacy`.

Available values:

* `random`
* `post`
* `websocket`

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

### Structure

```json
{
  "type": "sudoku",
  "tag": "sudoku-in",

  ... // Listen Fields

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

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### key

==Required==

Sudoku key.

It can be a shared secret string (same value on both sides) or an Ed25519 key.

To use Ed25519 key pair mode, generate one with `sing-box generate sudoku-keypair`, then set inbound `key` to `PublicKey` and outbound `key` to `PrivateKey`.

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

#### handshake_timeout

Handshake timeout in seconds.

#### disable_http_mask

Disable all HTTP masking layers.

#### http_mask_mode

HTTP masking mode.

Available values:

* `legacy` (write a fake HTTP header, not CDN compatible)
* `stream` (real HTTP streaming tunnel, CDN compatible)
* `poll` (real HTTP polling tunnel)
* `auto` (accept stream and poll)

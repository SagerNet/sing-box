# Introduction

sing-box uses JSON for configuration files.

### Structure

```json
{
  "log": {},
  "dns": {},
  "inbounds": [],
  "outbounds": [],
  "route": {},
  "experimental": {}
}
```

### Fields

| Key            | Format                         |
|----------------|--------------------------------|
| `log`          | [Log](./log)                   |
| `dns`          | [DNS](./dns)                   |
| `inbounds`     | [Inbound](./inbound)           |
| `outbounds`    | [Outbound](./outbound)         |
| `route`        | [Route](./route)               |
| `experimental` | [Experimental](./experimental) |

### Check

```bash
$ sing-box check
```

### Format

```bash
$ sing-box format -w
```
# Introduction

sing-box uses JSON for configuration files.

### Structure

```json
{
  "log": {},
  "dns": {},
  "ntp": {},
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
| `ntp`          | [NTP](./ntp)                   |
| `inbounds`     | [Inbound](./inbound)           |
| `outbounds`    | [Outbound](./outbound)         |
| `route`        | [Route](./route)               |
| `experimental` | [Experimental](./experimental) |

### Check

```bash
sing-box check
```

### Format

```bash
sing-box format -w -c config.json -D config_directory
```

### Merge

```bash
sing-box merge output.json -c config.json -D config_directory
```
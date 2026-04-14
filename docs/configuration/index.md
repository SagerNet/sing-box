# Introduction

sing-box uses JSON for configuration files.
### Structure

```json
{
  "log": {},
  "dns": {},
  "ntp": {},
  "certificate": {},
  "certificate_providers": [],
  "http_clients": [],
  "endpoints": [],
  "inbounds": [],
  "outbounds": [],
  "route": {},
  "services": [],
  "experimental": {}
}
```

### Fields

| Key            | Format                          |
|----------------|---------------------------------|
| `log`          | [Log](./log/)                   |
| `dns`          | [DNS](./dns/)                   |
| `ntp`          | [NTP](./ntp/)                   |
| `certificate`  | [Certificate](./certificate/)   |
| `certificate_providers` | [Certificate Provider](./shared/certificate-provider/) |
| `http_clients` | [HTTP Client](./shared/http-client/) |
| `endpoints`    | [Endpoint](./endpoint/)         |
| `inbounds`     | [Inbound](./inbound/)           |
| `outbounds`    | [Outbound](./outbound/)         |
| `route`        | [Route](./route/)               |
| `services`     | [Service](./service/)           |
| `experimental` | [Experimental](./experimental/) |

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

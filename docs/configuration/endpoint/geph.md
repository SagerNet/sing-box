`geph` endpoint launches Geph5 in packet/VPN mode and connects it to Sing-box through stdin/stdout. 

Geph5's stdio protocol is a sequence of raw IPv4/IPv6 packets, each prefixed by a 16-bit big-endian packet length. Sing-box provides the userspace IP stack that translates normal TCP and UDP endpoint operations into those packets.

### Structure

```json
{
  "type": "geph",
  "tag": "geph5",
  "executable_path": "/usr/bin/geph5-client",
  "config_path": "/etc/geph5/client.yaml",
  "control_address": "127.0.0.1:9913",
  "startup_timeout": "15s",
  "extra_args": []
}
```

### Fields

#### executable_path

Path to the Geph5 client executable.

Default: `geph5-client`.

#### config_path

Required path to the YAML configuration consumed by Geph5's `--config` option.

#### control_address

Address where Geph5 exposes its control RPC listener.

Required. Must be a loopback address (for example, `127.0.0.1:9913`).

Geph5 YAML must set `control_listen` to the same value. The address must be unused when the endpoint starts; Sing-box rejects an occupied listener so readiness cannot be reported by another Geph process. Geph5's TCP control RPC is unauthenticated, so keep it on loopback and protect local access to the host.

#### extra_args

Additional Geph5 command-line arguments. Sing-box always supplies `--config` and `--stdio-vpn`; those managed arguments must not be repeated here.

#### startup_timeout

Timeout for launching Geph5 and waiting for its control RPC `conn_info` state to become `Connected`. The endpoint is unavailable until Geph has established at least one authenticated tunnel session.

Default: `15s`.

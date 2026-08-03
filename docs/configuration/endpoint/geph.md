`geph` endpoint launches Geph5 in packet/VPN mode and connects it to Sing-box through stdin/stdout. 

Geph5's stdio protocol is a sequence of raw IPv4/IPv6 packets, each prefixed by a 16-bit big-endian packet length. Sing-box provides the userspace IP stack that translates normal TCP and UDP endpoint operations into those packets.

### Structure

```json
{
  "type": "geph",
  "tag": "geph5",
  "executable_path": "/usr/bin/geph5-client",
  "config_path": "/etc/geph5/client.yaml",
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

#### extra_args

Additional Geph5 command-line arguments. Sing-box always supplies `--config` and `--stdio-vpn`; those managed arguments must not be repeated here.

#### startup_timeout

Reserved startup timeout for the managed client process. The process must start successfully before the endpoint becomes available.

Default: `15s`.


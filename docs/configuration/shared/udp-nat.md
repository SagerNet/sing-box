---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [udp_mapping](#udp_mapping)  
    :material-plus: [udp_filtering](#udp_filtering)  
    :material-plus: [udp_nat_max](#udp_nat_max)

### Structure

```json
{
  "udp_timeout": "5m",
  "udp_mapping": "endpoint_independent",
  "udp_filtering": "endpoint_independent",
  "udp_nat_max": 0
}
```

### Fields

#### udp_timeout

UDP NAT expiration time.

`5m` will be used by default.

#### udp_mapping

!!! question "Since sing-box 1.14.0"

UDP NAT mapping behavior.

| Value                        | Behavior                                                                      |
|------------------------------|-------------------------------------------------------------------------------|
| `endpoint_independent`       | Reuse the same mapping for the same source address and port for all destinations. |
| `address_dependent`          | Use a separate mapping for each destination address.                          |
| `address_and_port_dependent` | Use a separate mapping for each destination address and port.                 |

`endpoint_independent` is used by default.

#### udp_filtering

!!! question "Since sing-box 1.14.0"

UDP NAT filtering behavior.

| Value                        | Behavior                                                                    |
|------------------------------|-----------------------------------------------------------------------------|
| `endpoint_independent`       | Accept packets from any remote endpoint.                                    |
| `address_dependent`          | Accept packets only from remote addresses to which packets have been sent.  |
| `address_and_port_dependent` | Accept packets only from remote addresses and ports to which packets have been sent. |

`endpoint_independent` is used by default.

#### udp_nat_max

!!! question "Since sing-box 1.14.0"

Maximum number of UDP NAT sessions.

When the limit is reached, the least recently used session is closed.

When unset or set to `0`, `4096` is used on iOS. On other platforms, a value from `4096` to `16384` is selected based on total memory;
`16384` is used if total memory cannot be detected.

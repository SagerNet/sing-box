!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [udp_mapping](/configuration/shared/udp-nat/#udp_mapping)  
    :material-plus: [udp_filtering](/configuration/shared/udp-nat/#udp_filtering)  
    :material-plus: [udp_nat_max](/configuration/shared/udp-nat/#udp_nat_max)

!!! quote ""

    Only supported on Linux.

### Structure

```json
{
  "type": "tproxy",
  "tag": "tproxy-in",

  ... // Listen Fields

  "network": "udp",

  ... // UDP NAT Fields
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### network

Listen network, one of `tcp` `udp`.

Both if empty.

### UDP NAT Fields

See [UDP NAT Fields](/configuration/shared/udp-nat/) for details.

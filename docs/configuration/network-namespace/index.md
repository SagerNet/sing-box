---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

!!! quote ""

    Only supported on Linux.

# Network Namespace

Network namespaces let inbounds and outbounds run inside a separate Linux network namespace,
referenced by tag from the [tun](/configuration/inbound/tun/#netns),
[Listen Fields](/configuration/shared/listen/#netns) and [Dial Fields](/configuration/shared/dial/#netns).

### Structure

```json
{
  "network_namespaces": [
    {
      "type": "",
      "tag": ""
    }
  ]
}
```

#### type

The type of the network namespace, `default` is used by default.

| Type      | Format                 |
|-----------|------------------------|
| `default` | [Default](./default/)  |
| `unshare` | [Unshare](./unshare/)  |

#### tag

==Required==

The tag of the network namespace.

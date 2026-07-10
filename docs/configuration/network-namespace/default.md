---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Default

Attach to an existing network namespace.

### Structure

```json
{
  "network_namespaces": [
    {
      "type": "default", // optional
      "tag": "",
      "path": ""
    }
  ]
}
```

### Fields

#### path

==Required==

Name or path of the network namespace, for example `sing` or `/run/netns/sing`.

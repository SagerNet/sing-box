---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Unshare

Create a new network namespace, without root privilege.

!!! info ""

    Rootless operation requires the kernel to allow unprivileged user namespace creation.

### Structure

```json
{
  "network_namespaces": [
    {
      "type": "unshare",
      "tag": "",
      "pid_file": ""
    }
  ]
}
```

### Fields

#### pid_file

If set, the PID of the process holding the namespace open is written to this path.

The namespace can be entered with `nsenter -t <pid> -n` when sing-box is run as root,
or `nsenter -t <pid> -U --preserve-credentials -n` otherwise.

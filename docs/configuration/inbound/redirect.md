---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: [auto_redirect](#auto_redirect)

!!! quote ""

    Only supported on Linux and macOS.

### Structure

```json
{
  "type": "redirect",
  "tag": "redirect-in",

  "auto_redirect": {
    "enabled": false,
    "continue_on_no_permission": false
  },

  ... // Listen Fields
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### `auto_redirect`

!!! question "Since sing-box 1.10.0"

!!! quote ""

    Only supported on Android.

Automatically add iptables nat rules to hijack **IPv4 TCP** connections.

It is expected to run with the Android graphical client (it will attempt to su at runtime).

#### `auto_redirect.continue_on_no_permission`

!!! question "Since sing-box 1.10.0"

Ignore errors when the Android device is not rooted or is denied root access.

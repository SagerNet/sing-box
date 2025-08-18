---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: [prefer_go](#prefer_go)  

!!! question "Since sing-box 1.12.0"

# Local

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "local",
        "tag": "",
        "prefer_go": false

        // Dial Fields
      }
    ]
  }
}
```

!!! info "Difference from legacy local server"

    * The old legacy local server only handles IP requests; the new one handles all types of requests and supports concurrent for IP requests.
    * The old local server uses default outbound by default unless detour is specified; the new one uses dialer just like outbound, which is equivalent to using an empty direct outbound by default.

### Fields

#### prefer_go

!!! question "Since sing-box 1.13.0"

When enabled, `local` DNS server will resolve DNS by dialing itself whenever possible.

Specifically, it disables following behaviors which was added as features in sing-box 1.13.0:

* On Apple platforms: Use `libresolv` for resolution, as it is the only one that works properly with NetworkExtension
  that overrides DNS servers (DHCP is also possible but is not considered).
* On Linux: Resolve through `systemd-resolvd`'s DBus interface when available.

As a sole exception, it cannot disable the following behavior:

In the Android graphical client, the `local` DNS server will always resolve DNS through the platform interface,
as there is no other way to obtain upstream DNS servers. 

On devices running Android versions lower than 10, this interface can only resolve IP queries.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# USB/IP Client

USB/IP Client service imports remote USB devices over [USB/IP](https://usbip.sourceforge.net/),
exported by the [USB/IP Server](/configuration/service/usbip-server/).

Available on Linux, Windows, and macOS (macOS requires a build with CGO). Not available on iOS.

The server must be a sing-box (or sing-usbip) server.

### Structure

```json
{
  "type": "usbip-client",

  ... // Dial Fields

  "server": "",
  "server_port": 0,
  "devices": []
}
```

!!! info "Difference from the official USB/IP protocol"

    sing-box uses [sing-usbip](https://github.com/SagerNet/sing-usbip), which uses an additional
    set of protocols to support enhancements such as hotplug, while remaining interoperable with
    the standard USB/IP protocol.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.

Only `detour` takes effect.

### Fields

#### server

==Required==

The remote `usbip-server` address.

#### server_port

The remote `usbip-server` port. Defaults to `3240`.

#### devices

List of device matches selecting which remote devices to import. If empty, all exported devices
are imported.

Object format:

```json
{
  "bus_id": "",
  "vendor_id": 0,
  "product_id": 0,
  "serial": ""
}
```

Object fields:

- `bus_id`: USB bus ID, e.g. `1-2`.
- `vendor_id`: USB vendor ID, as a number.
- `product_id`: USB product ID, as a number.
- `serial`: Device serial number.

Within one object, all specified fields must match; multiple objects are combined as a union. At
least one field is required.

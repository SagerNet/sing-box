---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# USB/IP Server

USB/IP Server service exports local USB devices over [USB/IP](https://usbip.sourceforge.net/),
to be imported by the [USB/IP Client](/configuration/service/usbip-client/) or a standard USB/IP
client.

Available on Linux, Windows, and macOS (macOS requires a build with CGO, and exporting devices
requires disabling System Integrity Protection). Not available on iOS.

### Structure

```json
{
  "type": "usbip-server",

  ... // Listen Fields

  "provider": "",
  "devices": []
}
```

!!! info "Difference from the official USB/IP protocol"

    sing-box uses [sing-usbip](https://github.com/SagerNet/sing-usbip), which uses an additional
    set of protocols to support enhancements such as hotplug, while remaining interoperable with
    the standard USB/IP protocol.

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

`listen_port` defaults to `3240`.

### Fields

#### provider

The device source provider.

- `default`: Exports the local devices matched by `devices`. The default value.
- `dynamic`: Devices are provided at runtime through a [sing-box API](/configuration/service/api/)
  client instead of from configuration, on supported platforms: the sing-box graphical clients on
  [macOS](/clients/apple/) and [Android](/clients/android/), and Chromium-based browsers with
  [sing-box Dashboard](https://github.com/SagerNet/sing-box-dashboard).

!!! quote ""

    The `default` provider is only supported when running directly via the CLI on Linux, Windows,
    and macOS, and requires elevated privileges.

#### devices

==Required== with the `default` provider.

List of device matches selecting which local USB devices to export.

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

---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# USB/IP Server

USB/IP Server 服务通过 [USB/IP](https://usbip.sourceforge.net/) 导出本地 USB 设备，供 [USB/IP Client](/zh/configuration/service/usbip-client/) 或标准 USB/IP 客户端导入。

可用于 Linux、Windows 和 macOS（macOS 需要使用 CGO 构建，且导出设备需要禁用系统完整性保护）。不支持 iOS。

### 结构

```json
{
  "type": "usbip-server",

  ... // 监听字段

  "provider": "",
  "devices": []
}
```

!!! info "与官方 USB/IP 协议的区别"

    sing-box 使用 [sing-usbip](https://github.com/SagerNet/sing-usbip)，它使用一套附加协议来支持热插拔等增强功能，但仍然可以与标准 USB/IP 互操作。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/) 了解详情。

`listen_port` 默认为 `3240`。

### 字段

#### provider

设备来源提供者。

- `default`：导出由 `devices` 匹配的本地设备。默认值。
- `dynamic`：设备在运行时通过 [sing-box API](/zh/configuration/service/api/) 客户端提供，而非来自配置文件，支持的平台包括 [macOS](/zh/clients/apple/) 和 [Android](/zh/clients/android/) 上的 sing-box 图形客户端，以及配合 [sing-box Dashboard](https://github.com/SagerNet/sing-box-dashboard) 的基于 Chromium 的浏览器。

!!! quote ""

    `default` 提供者仅支持通过 CLI 直接运行在 Linux、Windows 和 macOS 上，并且需要提升的权限。

#### devices

使用 `default` 提供者时 ==必填==。

设备匹配列表，用于选择要导出的本地 USB 设备。

对象格式：

```json
{
  "bus_id": "",
  "vendor_id": 0,
  "product_id": 0,
  "serial": ""
}
```

对象字段：

- `bus_id`：USB 总线 ID，例如 `1-2`。
- `vendor_id`：USB 供应商 ID，为数字。
- `product_id`：USB 产品 ID，为数字。
- `serial`：设备序列号。

在一个对象内，所有指定的字段都必须匹配；多个对象之间取并集。至少需要一个字段。

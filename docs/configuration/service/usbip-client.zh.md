---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# USB/IP Client

USB/IP Client 服务通过 [USB/IP](https://usbip.sourceforge.net/) 导入由 [USB/IP Server](/zh/configuration/service/usbip-server/) 导出的远程 USB 设备。

可用于 Linux、Windows 和 macOS（macOS 需要使用 CGO 构建）。不支持 iOS。

服务端必须是 sing-box（或 sing-usbip）服务端。

### 结构

```json
{
  "type": "usbip-client",

  ... // 拨号字段

  "server": "",
  "server_port": 0,
  "devices": []
}
```

!!! info "与官方 USB/IP 协议的区别"

    sing-box 使用 [sing-usbip](https://github.com/SagerNet/sing-usbip)，它使用一套附加协议来支持热插拔等增强功能，但仍然可以与标准 USB/IP 互操作。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。

仅 `detour` 生效。

### 字段

#### server

==必填==

远程 `usbip-server` 地址。

#### server_port

远程 `usbip-server` 端口。默认为 `3240`。

#### devices

设备匹配列表，用于选择要导入的远程设备。如果为空，则导入所有已导出的设备。

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

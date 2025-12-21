---
icon: material/new-box
---

# Wi-Fi 状态

!!! quote "sing-box 1.13.0 的变更"

    :material-plus: Linux 支持
    :material-plus: Windows 支持

sing-box 可以监控 Wi-Fi 状态，以启用基于 `wifi_ssid` 和 `wifi_bssid` 的路由规则。

### 平台支持

| 平台            | 支持              | 备注           |
|-----------------|------------------|----------------|
| Android         | :material-check: | 仅图形客户端    |
| Apple 平台      | :material-check: | 仅图形客户端    |
| Linux           | :material-check: | 需要支持的守护进程 |
| Windows         | :material-check: | WLAN API       |
| 其他            | :material-close: |                |

### Linux

!!! question "自 sing-box 1.13.0 起"

支持以下后端，将按优先级顺序自动探测：

| 后端              | 接口         |
|------------------|-------------|
| NetworkManager   | D-Bus       |
| IWD              | D-Bus       |
| wpa_supplicant   | Unix socket |
| ConnMan          | D-Bus       |

### Windows

!!! question "自 sing-box 1.13.0 起"

使用 Windows WLAN API。

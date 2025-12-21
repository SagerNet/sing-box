---
icon: material/new-box
---

# Wi-Fi State

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: Linux support
    :material-plus: Windows support

sing-box can monitor Wi-Fi state to enable routing rules based on `wifi_ssid` and `wifi_bssid`.

### Platform Support

| Platform        | Support          | Notes                    |
|-----------------|------------------|--------------------------|
| Android         | :material-check: | In graphical client      |
| Apple platforms | :material-check: | In graphical clients     |
| Linux           | :material-check: | Requires supported daemon |
| Windows         | :material-check: | WLAN API                 |
| Others          | :material-close: |                          |

### Linux

!!! question "Since sing-box 1.13.0"

The following backends are supported and will be auto-detected in order of priority:

| Backend          | Interface   |
|------------------|-------------|
| NetworkManager   | D-Bus       |
| IWD              | D-Bus       |
| wpa_supplicant   | Unix socket |
| ConnMan          | D-Bus       |

### Windows

!!! question "Since sing-box 1.13.0"

Uses Windows WLAN API.

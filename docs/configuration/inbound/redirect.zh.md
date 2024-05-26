---
icon: material/new-box
---

!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: [auto_redirect](#auto_redirect)

!!! quote ""

    仅支持 Linux 和 macOS。

### 结构

```json
{
  "type": "redirect",
  "tag": "redirect-in",

  "auto_redirect": {
    "enabled": false,
    "continue_on_no_permission": false
  },
  
  ... // 监听字段
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### `auto_redirect`

!!! question "自 sing-box 1.10.0 起"

!!! quote ""

    仅支持 Android。

自动添加 iptables nat 规则以劫持 **IPv4 TCP** 连接。

它预计与 Android 图形客户端一起运行（将在运行时尝试 su）。

#### `auto_redirect.continue_on_no_permission`

!!! question "自 sing-box 1.10.0 起"

当 Android 设备未获得 root 权限或 root 访问权限被拒绝时，忽略错误。

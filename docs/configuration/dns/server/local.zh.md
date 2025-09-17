---
icon: material/new-box
---

!!! quote "sing-box 1.13.0 中的更改"

    :material-plus: [prefer_go](#prefer_go)

!!! question "自 sing-box 1.12.0 起"

# Local

### 结构

```json
{
  "dns": {
    "servers": [
      {
        "type": "local",
        "tag": "",
        "prefer_go": false,

        // 拨号字段
      }
    ]
  }
}
```

!!! info "与旧版本地服务器的区别"

    * 旧的传统本地服务器只处理 IP 请求；新的服务器处理所有类型的请求，并支持 IP 请求的并发处理。
    * 旧的本地服务器默认使用默认出站，除非指定了绕行；新服务器像出站一样使用拨号器，相当于默认使用空的直连出站。

### 字段

#### prefer_go

!!! question "自 sing-box 1.13.0 起"

启用后，`local` DNS 服务器将尽可能通过拨号自身来解析 DNS。

具体来说，它禁用了在 sing-box 1.13.0 中作为功能添加的以下行为：

1. 在 Apple 平台上：尝试在 NetworkExtension 中使用 `getaddrinfo` 解析 A/AAAA 请求。
2. 在 Linux 上：当可用时通过 `systemd-resolvd` 的 DBus 接口进行解析。

作为唯一的例外，它无法禁用以下行为：

1. 在 Android 图形客户端中，
`local` 将始终通过平台接口解析 DNS，
因为没有其他方法来获取上游 DNS 服务器；
在运行 Android 10 以下版本的设备上，此接口只能解析 A/AAAA 请求。

2. 在 macOS 上，`local` 会在 Network Extension 中首先尝试 DHCP，由于 DHCP 遵循拨号字段，
它不会被 `prefer_go` 禁用。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/) 了解详情。
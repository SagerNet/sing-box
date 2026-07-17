# OpenConnect 客户端

!!! question "自 sing-box 1.14.0 起"

==仅客户端==

## 结构

```json
{
  "type": "openconnect",
  "tag": "oc-client",

  "system": false,
  "name": "",
  "server": "vpn.example.com",
  "flavor": "anyconnect",
  "username": "",
  "password": "",
  "auth_group": "",
  "token": {
    "mode": "",
    "secret": "",
    "pin": "",
    "password": "",
    "device_id": "",
    "counter": 0
  },
  "reported_os": "",
  "user_agent": "",
  "csd": {
    "wrapper_path": ""
  },
  "hip": {
    "wrapper_path": ""
  },
  "tncc": {
    "wrapper_path": "",
    "device_id": "",
    "user_agent": "",
    "machine_identification_enabled": false,
    "certificates": [
      {
        "certificate": [],
        "certificate_path": ""
      }
    ]
  },
  "no_udp": false,
  "allow_insecure_crypto": false,
  "tls": {
    "certificate_authority": [],
    "certificate_authority_path": "",
    "client_certificate": [],
    "client_certificate_path": "",
    "client_key": [],
    "client_key_path": "",
    "client_key_password": "",
    "mca_certificate": [],
    "mca_certificate_path": "",
    "mca_key": [],
    "mca_key_path": "",
    "mca_key_password": ""
  },
  "form_entries": [
    {
      "form_id": "",
      "submission_key": "",
      "name": "",
      "value": "",
      "promote": false
    }
  ],

  ... // 拨号字段
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

## 字段

### system

使用系统接口。

需要权限，且不能与现有系统接口冲突。

禁用时，sing-box 使用内部网络栈。

### name

系统接口的自定义接口名称。

默认使用自动生成的 `oc` 接口名称。

### server

==必填==

OpenConnect VPN 服务器 HTTPS URL。

省略协议时会添加 `https://`。不支持 URL 用户信息、查询和片段。

### flavor

OpenConnect 协议 flavor，可选值为 `anyconnect`、`gp`、`fortinet`、`f5`、`pulse` 或 `nc`。

默认使用 `anyconnect`。

### username

用于填充匹配认证表单字段的用户名。

### password

用于填充匹配认证表单字段的密码。

### auth_group

认证组，用于在所选 flavor 支持时预选匹配的组、realm、domain 或 gateway 选项。

### token

用于自动回答匹配 token 字段的软件 token 配置。

### token.mode

==必填==

软件 token 模式，可选值为：

- `totp`：基于时间的一次性密码。
- `hotp`：基于 HMAC 的一次性密码。
- `stoken`：RSA SecurID 软件 token。

### token.secret

==必填==

软件 token 密钥。

对于 `totp` 和 `hotp`，可以是 Base32 密钥、带 `base32:` 前缀的密钥或类型匹配的 `otpauth://` URI。

对于 `stoken`，这是编码后的 RSA SecurID CTF token 内容。

### token.pin

`stoken` 模式的 RSA SecurID PIN。

### token.password

`stoken` 模式下用于解密受密码保护的 RSA SecurID token 的密码。

### token.device_id

`stoken` 模式下用于解密设备绑定 RSA SecurID token 的设备 ID。

### token.counter

`hotp` 模式的初始计数器。

为零时，如果 `otpauth://` URI 中存在计数器，则使用该计数器；否则从零开始。

### reported_os

所选 flavor 支持时向 VPN 服务器报告的操作系统标识。

对于 `anyconnect`、`gp` 和 `pulse`，支持的值为 `linux`、`linux-64`、`win`、`mac-intel`、`android` 和 `apple-ios`。

`anyconnect` 默认使用 `linux-64`。`gp` 和 `pulse` 默认根据系统平台选择值。

### user_agent

所选 flavor 支持时向 VPN 服务器报告的 User-Agent。

默认值由 flavor 决定。

### csd

AnyConnect CSD/host scan 合规性选项。

服务器请求 CSD 时，默认使用内置 CSD 处理。

### csd.wrapper_path

外部 AnyConnect CSD wrapper 可执行文件的路径。

为空时使用内置 CSD 处理。

### hip

GlobalProtect HIP 检查和报告选项。

服务器请求 HIP 时，默认使用内置 HIP 报告。

### hip.wrapper_path

外部 GlobalProtect HIP report wrapper 可执行文件的路径。

为空时使用内置 HIP 报告。

### tncc

Network Connect TNCC 合规性选项。

服务器请求 TNCC 时，默认使用内置 TNCC 处理。

### tncc.wrapper_path

外部 Network Connect TNCC wrapper 可执行文件的路径。

为空时使用内置 TNCC 处理。

与 `tncc.device_id`、`tncc.user_agent`、`tncc.machine_identification_enabled` 和 `tncc.certificates` 冲突。

### tncc.device_id

内置 TNCC 处理程序报告的设备 ID。

与 `tncc.wrapper_path` 冲突。

### tncc.user_agent

内置 TNCC 处理程序使用的 User-Agent。

默认使用 `Neoteris HC Http`。

与 `tncc.wrapper_path` 冲突。

### tncc.machine_identification_enabled

启用内置 TNCC 机器标识，包括平台、主机名和观测到的 MAC 地址。

与 `tncc.wrapper_path` 冲突。

### tncc.certificates

内置 TNCC 处理程序用于回答证书请求的机器证书。

需要启用 `tncc.machine_identification_enabled`。

与 `tncc.wrapper_path` 冲突。

### tncc.certificates.certificate

PEM 格式的 TNCC 机器证书内容。

与 `tncc.certificates.certificate_path` 冲突。

### tncc.certificates.certificate_path

PEM 格式的 TNCC 机器证书路径。

与 `tncc.certificates.certificate` 冲突。

### no_udp

禁用 DTLS 或 ESP 辅助数据通道，仅使用 TLS 数据通道。

### allow_insecure_crypto

允许旧版 VPN 服务器所需的已弃用 TLS 和 DTLS 版本及密码套件。

默认禁用。此选项不会禁用服务器证书验证。

### tls

OpenConnect TLS 配置。

### tls.certificate_authority

PEM 格式的附加受信任 CA 证书内容。

这些证书会添加到系统证书池。

与 `tls.certificate_authority_path` 冲突。

### tls.certificate_authority_path

PEM 格式的附加受信任 CA 证书路径。

这些证书会添加到系统证书池。

与 `tls.certificate_authority` 冲突。

### tls.client_certificate

PEM 格式的客户端证书链内容。

与 `tls.client_certificate_path` 冲突。

### tls.client_certificate_path

PEM 格式的客户端证书链路径。

与 `tls.client_certificate` 冲突。

### tls.client_key

PEM 格式的客户端私钥内容。

与 `tls.client_key_path` 冲突。

### tls.client_key_path

PEM 格式的客户端私钥路径。

与 `tls.client_key` 冲突。

客户端证书和私钥必须同时设置或同时为空。

### tls.client_key_password

加密客户端私钥的密码。

### tls.mca_certificate

PEM 格式的 AnyConnect 多证书认证（MCA）证书链内容。

与 `tls.mca_certificate_path` 冲突。

### tls.mca_certificate_path

PEM 格式的 AnyConnect 多证书认证（MCA）证书链路径。

与 `tls.mca_certificate` 冲突。

### tls.mca_key

PEM 格式的 AnyConnect 多证书认证（MCA）私钥内容。

与 `tls.mca_key_path` 冲突。

### tls.mca_key_path

PEM 格式的 AnyConnect 多证书认证（MCA）私钥路径。

与 `tls.mca_key` 冲突。

MCA 证书和私钥必须同时设置或同时为空。

### tls.mca_key_password

加密 MCA 私钥的密码。

### form_entries

认证表单字段覆盖。

设置 `submission_key` 时按该字段匹配，否则按 `form_id` 和 `name` 的组合匹配。后面的匹配项优先。

### form_entries.form_id

`form_entries.submission_key` 为空时，与 `form_entries.name` 一起使用的认证表单标识符。

### form_entries.submission_key

认证字段提交键。

`form_entries.submission_key` 或 `form_entries.form_id` 与 `form_entries.name` 的组合之一必填。

### form_entries.name

`form_entries.submission_key` 为空时，与 `form_entries.form_id` 一起使用的认证字段名称。

### form_entries.value

自动提供给匹配认证字段的值。

与 `form_entries.promote` 冲突。

### form_entries.promote

交互询问匹配的认证字段，而不是自动提供值。

与 `form_entries.value` 冲突。

## 拨号字段

参阅[拨号字段](/zh/configuration/shared/dial/)了解详情。

## 交互式认证

在 sing-box dashboard 或任意 sing-box 图形客户端的 `工具` > `端点` 中认证和管理 endpoint。

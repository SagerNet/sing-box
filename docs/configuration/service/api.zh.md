---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# sing-box API

sing-box API 服务是用于观察与控制正在运行的 sing-box 实例的 gRPC 服务器。

它可以由 iOS、macOS 和 Android 上的 [sing-box 图形客户端](/zh/clients/)（通过 Remote Control 功能）或 [sing-box dashboard](https://github.com/SagerNet/sing-box-dashboard) 访问。

服务器同时接受 [gRPC-Web](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md) 请求。

### 结构

```json
{
  "type": "api",
  
  ... // 监听字段
  
  "secret": "",
  "access_control_allow_origin": [],
  "access_control_allow_private_network": false,
  "dashboard": {
    "enabled": true,
    "path": "",
    "download_url": "",
    "http_client": "", // 或 {}
    "update_interval": ""
  },
  "tls": {}
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### secret

API 密钥。

客户端通过标准的 `authorization: Bearer <secret>` gRPC metadata 头认证。

默认无需认证。

#### access_control_allow_origin

允许的 CORS 来源,默认使用 `*`。

#### access_control_allow_private_network

允许从私有网络访问。

#### dashboard

下载并通过 API 监听器在 `/dashboard/` 提供的 Web 仪表板；其他浏览器请求将被重定向到该路径。

!!! info ""

    该对象可以替换为布尔值（等同于 `{ "enabled": <bool> }`），
    或字符串路径（等同于 `{ "enabled": true, "path": "<string>" }`）。

##### enabled

启用仪表板。

##### path

存放仪表板文件的目录。

默认使用工作目录下的 `dashboard`。

如果目录中包含非 sing-box 下载的文件，将按原样提供。

##### download_url

仪表板压缩包（zip）的下载 URL。

默认使用 `https://github.com/SagerNet/sing-box-dashboard/archive/refs/heads/gh-pages.zip`。

##### http_client

用于下载仪表板的 HTTP 客户端。

参阅 [HTTP 客户端字段](/zh/configuration/shared/http-client/)。

##### update_interval

仪表板的更新间隔。

默认使用 `1d`。

#### tls

TLS 配置,参阅 [TLS](/zh/configuration/shared/tls/#inbound)。

# 实验性

### 结构

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "",
      "external_ui_download_url": "",
      "external_ui_download_detour": "",
      "secret": "",
      "default_mode": "",
      "store_mode": false,
      "store_selected": false,
      "store_fakeip": false,
      "cache_file": "",
      "cache_id": ""
    },
    "v2ray_api": {
      "listen": "127.0.0.1:8080",
      "stats": {
        "enabled": true,
        "inbounds": [
          "socks-in"
        ],
        "outbounds": [
          "proxy",
          "direct"
        ],
        "users": [
          "sekai"
        ]
      }
    }
  }
}
```

!!! note ""

    流量统计和连接管理会降低性能。

### Clash API 字段

!!! error ""

    默认安装不包含 Clash API，参阅 [安装](/zh/#_2)。

#### external_controller

RESTful web API 监听地址。如果为空，则禁用 Clash API。

#### external_ui

到静态网页资源目录的相对路径或绝对路径。sing-box 会在 `http://{{external-controller}}/ui` 下提供它。

#### external_ui_download_url

静态网页资源的 ZIP 下载 URL，如果指定的 `external_ui` 目录为空，将使用。

默认使用 `https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip`。

#### external_ui_download_detour

用于下载静态网页资源的出站的标签。

如果为空，将使用默认出站。

#### secret

RESTful API 的密钥（可选）
通过指定 HTTP 标头 `Authorization: Bearer ${secret}` 进行身份验证
如果 RESTful API 正在监听 0.0.0.0，请始终设置一个密钥。

#### default_mode

Clash 中的默认模式，默认使用 `rule`。

此设置没有直接影响，但可以通过 `clash_mode` 规则项在路由和 DNS 规则中使用。

#### store_mode

将 Clash 模式存储在缓存文件中。

#### store_selected

!!! note ""

    必须为目标出站设置标签。

将 `Selector` 中出站的选定的目标出站存储在缓存文件中。

#### store_fakeip

将 fakeip 存储在缓存文件中。

#### cache_file

缓存文件路径，默认使用`cache.db`。

#### cache_id

缓存 ID。

如果不为空，`store_selected` 将会使用以此为键的独立存储。

### V2Ray API 字段

!!! error ""

    默认安装不包含 V2Ray API，参阅 [安装](/zh/#_2)。

#### listen

gRPC API 监听地址。如果为空，则禁用 V2Ray API。

#### stats

流量统计服务设置。

#### stats.enabled

启用统计服务。

#### stats.inbounds

统计流量的入站列表。

#### stats.outbounds

统计流量的出站列表。

#### stats.users

统计流量的用户列表。
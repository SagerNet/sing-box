# 实验性

### 结构

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "folder",
      "secret": "",
      "default_mode": "rule",
      "store_selected": false,
      "cache_file": "cache.db"
    }
  }
}
```

### Clash API 字段

!!! error ""

    默认安装不包含 Clash API，参阅 [安装](/zh/#_2)。

!!! note ""

    流量统计和连接管理将禁用 Linux 中的 TCP splice 并降低性能，使用风险自负。

#### external_controller

RESTful web API 监听地址。如果为空，则禁用 Clash API。

#### external_ui

到静态网页资源目录的相对路径或绝对路径。sing-box 会在 `http://{{external-controller}}/ui` 下提供它。

#### secret

RESTful API 的密钥（可选）
通过指定 HTTP 标头 `Authorization: Bearer ${secret}` 进行身份验证
如果 RESTful API 正在监听 0.0.0.0，请始终设置一个密钥。

#### default_mode

Clash 中的默认模式，默认使用 `rule`。

此设置没有直接影响，但可以通过 `clash_mode` 规则项在路由和 DNS 规则中使用。

#### store_selected

!!! note ""

    必须为目标出站设置标签。

将 `Selector` 中出站的选定的目标出站存储在缓存文件中。

#### cache_file

缓存文件路径，默认使用`cache.db`。
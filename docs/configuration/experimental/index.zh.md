# 实验性

### 结构

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "folder",
      "secret": ""
    }
  }
}
```

### Clash API 字段

!!! error ""

    默认安装不包含 Clash API，参阅 [安装](/zh/#installation)。

!!! note ""

    流量统计和连接管理将禁用 Linux 中的 TCP splice 并降低性能，使用风险自负。

#### external_controller

RESTful web API 监听地址。

#### external_ui

到静态网页资源目录的相对路径或绝对路径。sing-box 会在 `http://{{external-controller}}/ui` 下提供它。

#### secret

RESTful API 的密钥（可选）
通过指定 HTTP 标头 `Authorization: Bearer ${secret}` 进行身份验证
如果 RESTful API 正在监听 0.0.0.0，请始终设置一个密钥。
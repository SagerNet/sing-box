---
icon: material/alert-decagram
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [bbr_profile](#bbr_profile)  
    :material-plus: [realm](#realm)

!!! quote "sing-box 1.11.0 中的更改"

    :material-alert: [masquerade](#masquerade)  
    :material-alert: [ignore_client_bandwidth](#ignore_client_bandwidth)

### 结构

```json
{
  "type": "hysteria2",
  "tag": "hy2-in",
  
  ... // 监听字段

  "up_mbps": 100,
  "down_mbps": 100,
  "obfs": {
    "type": "salamander",
    "password": "cry_me_a_r1ver"
  },
  "users": [
    {
      "name": "tobyxdd",
      "password": "goofy_ahh_password"
    }
  ],
  "ignore_client_bandwidth": false,
  "tls": {},

  ... // QUIC 字段

  "masquerade": "", // 或 {}
  "bbr_profile": "",
  "brutal_debug": false,
  "realm": {
    "server_url": "https://realm.example.com",
    "token": "",
    "realm_id": "",
    "stun_servers": [],
    "stun_domain_resolver": "", // 或 {}
    "http_client": {}
  }
}
```

!!! warning "与官方 Hysteria2 的区别"

    官方程序支持一种名为 **userpass** 的验证方式，
    本质上是将用户名与密码的组合 `<username>:<password>` 作为实际上的密码，而 sing-box 不提供此别名。
    要将 sing-box 与官方程序一起使用， 您需要填写该组合作为实际密码。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### up_mbps, down_mbps

支持的速率，默认不限制。

与 `ignore_client_bandwidth` 冲突。

#### obfs.type

QUIC 流量混淆器类型，仅可设为 `salamander`。

如果为空则禁用。

#### obfs.password

QUIC 流量混淆器密码.

#### users

Hysteria 用户

#### users.password

认证密码。

#### ignore_client_bandwidth

*当 `up_mbps` 和 `down_mbps` 未设定时*:

命令客户端使用 BBR 拥塞控制算法而不是 Hysteria CC。

*当 `up_mbps` 和 `down_mbps` 已设定时*:

禁止客户端使用 BBR 拥塞控制算法。

#### tls

==必填==

TLS 配置, 参阅 [TLS](/zh/configuration/shared/tls/#入站)。

### QUIC 字段

参阅 [QUIC 字段](/zh/configuration/shared/quic/) 了解详情。

#### masquerade

HTTP3 服务器认证失败时的行为 （URL 字符串配置）。

| Scheme       | 示例                      | 描述      |
|--------------|-------------------------|---------|
| `file`       | `file:///var/www`       | 作为文件服务器 |
| `http/https` | `http://127.0.0.1:8080` | 作为反向代理  |

如果 masquerade 未配置，则返回 404 页。

与 `masquerade.type` 冲突。

#### masquerade.type

HTTP3 服务器认证失败时的行为 （对象配置）。

| Type     | 描述      | 字段                                  |
|----------|---------|-------------------------------------|
| `file`   | 作为文件服务器 | `directory`                         |
| `proxy`  | 作为反向代理  | `url`, `rewrite_host`               |
| `string` | 返回固定响应  | `status_code`, `headers`, `content` |

如果 masquerade 未配置，则返回 404 页。

与 `masquerade` 冲突。

#### masquerade.directory

文件服务器根目录。

#### masquerade.url

反向代理目标 URL。

#### masquerade.rewrite_host

重写请求头中的 Host 字段到目标 URL。

#### masquerade.status_code

固定响应状态码。

#### masquerade.headers

固定响应头。

#### masquerade.content

固定响应内容。

#### bbr_profile

!!! question "自 sing-box 1.14.0 起"

BBR 拥塞控制算法配置，可选 `conservative` `standard` `aggressive`。

默认使用 `standard`。

#### brutal_debug

启用 Hysteria Brutal CC 的调试信息日志记录。

#### realm

!!! question "自 sing-box 1.14.0 起"

将此入站注册到 Hysteria Realm 会合服务，以启用 NAT 穿透。

入站通过 STUN 发现自己的公网地址并注册到 realm，借助 UDP 打洞接受客户端连接，无需可公网直达的监听地址。

会合服务参阅 [Hysteria Realm](/zh/configuration/service/hysteria-realm/)。

#### realm.server_url

==必填==

Realm 会合服务 URL。

#### realm.token

Realm 的 Bearer 令牌，需与 realm 上配置的 `users[].token` 之一匹配。

#### realm.realm_id

==必填==

Realm 上的槽位标识符。

1–64 字符，需匹配 `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`。

出站需使用相同的 `realm_id` 才能找到本服务器。

#### realm.stun_servers

==必填==

用于发现公网地址的 STUN 服务器列表（`host` 或 `host:port`）。

#### realm.stun_domain_resolver

用于解析 STUN 服务器域名的域名解析器。

此选项的格式与 [路由 DNS 规则动作](/zh/configuration/dns/rule_action/#route) 相同，但不包含 `action` 字段。

若直接将此选项设置为字符串，则等同于设置该选项的 `server` 字段。

如果为空，则使用默认域名解析器。

#### realm.http_client

与 realm 通信使用的 HTTP 客户端。

参阅 [HTTP 客户端](/zh/configuration/shared/http-client/) 了解详情。

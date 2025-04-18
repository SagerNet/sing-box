# sing-box

The universal proxy platform.

[![Packaging status](https://repology.org/badge/vertical-allrepos/sing-box.svg)](https://repology.org/project/sing-box/versions)

## Documentation

https://sing-box.sagernet.org

## Dynamic API

使用 Dynamic API 功能，你可以在运行时动态管理入站、出站和路由规则。

### 编译时启用 Dynamic API

在编译时添加 `with_dynamic_api` 标签：

```bash
go build -tags "with_dynamic_api" ./cmd/sing-box
```

或者修改 `Makefile` 中的 `TAGS` 变量：

```
TAGS ?= with_gvisor,with_dhcp,with_wireguard,with_reality_server,with_clash_api,with_quic,with_utls,with_tailscale,with_dynamic_api
```

### 配置文件示例

在配置文件的 `experimental` 部分添加：

```json
{
  "experimental": {
    "dynamic_api": {
      "listen": "127.0.0.1:9090",
      "secret": "your_api_secret"
    }
  }
}
```

### API 用法

Dynamic API 提供以下功能：

- 动态添加/删除入站
- 动态添加/删除出站
- 动态添加/删除路由规则

详细 API 文档请参考官方文档。

### 使用示例

#### 动态添加入站

```bash
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/inbound \
  -d '{
    "tag": "http_in",
    "type": "http",
    "listen": "127.0.0.1",
    "listen_port": 10080,
    "users": [
      {
        "username": "user",
        "password": "pass"
      }
    ]
  }'
```

#### 删除入站

```bash
curl -X DELETE -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/inbound/http_in
```

#### 列出所有入站

```bash
curl -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/inbound
```

#### 动态添加出站

```bash
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/outbound \
  -d '{
    "tag": "proxy_out",
    "type": "vmess",
    "server": "example.com",
    "server_port": 443,
    "security": "auto",
    "uuid": "bf000d23-0752-40b4-affe-68f7707a9661",
    "alter_id": 0,
    "tls": {
      "enabled": true,
      "server_name": "example.com"
    }
  }'
#
{
  "type": "vless",
  "tag": "proxy1",
  "server": "shen.86782889.xyz",
  "server_port": 55536,
  "uuid": "1ecd415e-6b5a-5988-c66f-2e67f28d1e72",
  "flow": "",
  "network": "tcp",
  "tls": {
    "enabled": true,
    "server_name": "shen.86782889.xyz",
    "insecure": false
  },
  "transport": {
    "type": "ws",
    "path": "/SHDsdfsjk2365"
  }
}
#

```

#### 删除出站

```bash
curl -X DELETE -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/outbound/proxy_out
```

#### 动态添加路由规则

```bash
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/route/rule \
  -d '{
    "domain": ["example.com"],
    "outbound": "proxy_out"
  }'
```

也可以根据进程名添加规则：

```bash
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/route/rule \
  -d '{
    "process_name": ["ip2.exe"],
    "outbound": "socks-out1"
  }'
```

或根据进程PID添加规则：

```bash
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/route/rule \
  -d '{
    "process_pid": [13588],
    "outbound": "proxy1"
  }'
```

#### 删除路由规则

路由规则通过索引删除，首先可以列出所有规则：

```bash
curl -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/route/rules
```

然后删除指定索引的规则（例如删除索引为0的规则）：

```bash
curl -X DELETE -H "Authorization: Bearer your_api_secret" \
  http://127.0.0.1:9090/api/route/rule/0
```

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

In addition, no derivative work may use the name or imply association
with this application without prior consent.
# Dynamic API 使用指南

## 配置说明

在 `config.json` 中启用 Dynamic API：

```json
{
    "log": {
        "level": "info",
        "timestamp": true
    },
    "inbounds": [
        {
            "type": "tun",
            "tag": "tun-in",
            "interface_name": "sing-box",
            "inet4_address": "172.19.0.1/30",
            "auto_route": true,
            "stack": "system",
            "sniff": true
        }
    ],
    "outbounds": [
        {
            "type": "direct",
            "tag": "direct"
        },
        {
            "type": "block",
            "tag": "block"
        }
    ],
    "route": {
        "auto_detect_interface": true,
        "rules": [
            {
                "action": "sniff"
            },
            {
                "protocol": "dns",
                "action": "hijack-dns"
            }
        ],
        "find_process": true,
        "final": "direct"
    },
    "experimental": {
        "dynamic_api": {
            "listen": "127.0.0.1:9091",
            "secret": "your_secret_key111111",
            "enable_config_save": true,
            "config_save_path": "dynamic_config.json"
        }
    }
}
```

参数说明：
- `listen`: API 服务监听地址
- `secret`: API 访问密钥，用于认证
- `enable_config_save`: 是否启用配置保存功能
- `config_save_path`: 动态配置保存路径

## API 端点

### 入站管理

#### 创建入站
```bash
curl -X POST http://127.0.0.1:9091/api/inbound \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "http",
    "tag": "http-in",
    "listen": "127.0.0.1",
    "listen_port": 1080
  }'
```

#### 删除入站
```bash
curl -X DELETE http://127.0.0.1:9091/api/inbound/http-in \
  -H "Authorization: your_secret_key"
```

#### 列出所有入站
```bash
curl http://127.0.0.1:9091/api/inbound \
  -H "Authorization: your_secret_key"
```

### 出站管理

#### 创建出站
```bash
# VLESS 出站示例
curl -X POST http://127.0.0.1:9091/api/outbound \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "vless",
    "tag": "proxy1",
    "server": "example.com",
    "server_port": 443,
    "uuid": "1234567890-abcd-efgh-ijkl",
    "flow": "",
    "network": "tcp",
    "tls": {
      "enabled": true,
      "server_name": "example.com",
      "insecure": false
    },
    "transport": {
      "type": "ws",
      "path": "/path"
    }
  }'

# Shadowsocks 出站示例
curl -X POST http://127.0.0.1:9091/api/outbound \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shadowsocks",
    "tag": "ss-out",
    "server": "example.com",
    "server_port": 8388,
    "method": "aes-256-gcm",
    "password": "your_password"
  }'
```

#### 删除出站
```bash
curl -X DELETE http://127.0.0.1:9091/api/outbound/proxy1 \
  -H "Authorization: your_secret_key"
```

#### 列出所有出站
```bash
curl http://127.0.0.1:9091/api/outbound \
  -H "Authorization: your_secret_key"
```

#### 测试出站连接
```bash
curl -X POST http://127.0.0.1:9091/api/outbound/test \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "tag": "proxy1",
    "test_url": "http://www.gstatic.com/generate_204"
  }'
```

### 路由规则管理

#### 创建路由规则
```bash
# 进程规则示例
curl -X POST http://127.0.0.1:9091/api/route/rule \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "process_name": ["chrome.exe"],
    "outbound": "proxy1"
  }'

# 域名规则示例
curl -X POST http://127.0.0.1:9091/api/route/rule \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": ["example.com", "*.example.com"],
    "outbound": "proxy1"
  }'

# IP 规则示例
curl -X POST http://127.0.0.1:9091/api/route/rule \
  -H "Authorization: your_secret_key" \
  -H "Content-Type: application/json" \
  -d '{
    "ip_cidr": ["192.168.1.0/24", "10.0.0.0/8"],
    "outbound": "proxy1"
  }'
```

#### 删除路由规则
```bash
curl -X DELETE http://127.0.0.1:9091/api/route/rule/0 \
  -H "Authorization: your_secret_key"
```

#### 列出所有规则
```bash
curl http://127.0.0.1:9091/api/route/rules \
  -H "Authorization: your_secret_key"
```

### 配置管理

#### 保存配置
```bash
curl -X POST http://127.0.0.1:9091/api/config/save \
  -H "Authorization: your_secret_key"
```

#### 重载配置
```bash
curl -X POST http://127.0.0.1:9091/api/config/reload \
  -H "Authorization: your_secret_key"
```

## 响应格式

### 成功响应
```json
{
    "success": true,
    "message": "操作成功",
    "data": {}  // 可选，包含返回数据
}
```

### 错误响应
```json
{
    "error": "错误信息"
}
```

## 注意事项

1. 所有请求都需要在 Header 中包含 `Authorization` 字段，值为配置中的 `secret`
2. 删除操作会同步更新到配置文件（如果启用了配置保存功能）
3. 初始配置（来自 config.json）中的入站和出站不会被保存到动态配置文件中
4. 系统入站（如 tun）不会被包含在动态配置中
5. 配置保存功能需要在配置中明确启用（`enable_config_save: true`） 
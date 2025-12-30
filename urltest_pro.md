# urltest_pro 使用说明

## 概述

`urltest_pro` 是基于 `urltest` 扩展的智能节点选择协议，在原有延迟测试的基础上增加了**权重选择机制**，让用户可以更灵活地控制节点优先级。

### 核心算法

| 协议 | 选择逻辑 |
|------|----------|
| `urltest` | 选择延迟最低的节点 |
| `urltest_pro` | 选择 `延迟/权重` 分数最低的节点 |

**公式**: `score = delay / weight`

- 分数越低，节点越优先
- 权重越高，节点越容易被选中
- 相同延迟下，权重高的节点优先

## 配置参数

### outbound 权重配置

在任意 outbound 的顶层配置中添加 `weight` 字段：

```json
{
  "type": "vmess",
  "tag": "node-hk",
  "weight": 2.0,
  "server": "example.com",
  ...
}
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `weight` | float64 | 1.0 | 节点权重，值越大优先级越高 |

**特殊值**:
- 未配置或 `weight: 1.0` - 默认权重，与原 urltest 行为一致
- `weight: 0` - 禁用该节点，不参与选择
- `weight: 2.0` - 双倍权重，相当于延迟减半

### urltest_pro 配置

```json
{
  "type": "urltest_pro",
  "tag": "auto-select",
  "outbounds": ["node-hk", "node-jp", "node-us"],
  "url": "https://www.gstatic.com/generate_204",
  "interval": "3m",
  "tolerance": 50,
  "idle_timeout": "30m",
  "interrupt_exist_connections": false
}
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `outbounds` | []string | 必填 | 参与选择的出站节点列表 |
| `url` | string | `https://www.gstatic.com/generate_204` | 测速 URL |
| `interval` | duration | `3m` | 测速间隔 |
| `tolerance` | uint16 | `0` | 容差值(ms)，分数差异在此范围内不切换 |
| `idle_timeout` | duration | `30m` | 空闲超时，无连接时暂停测速 |
| `interrupt_exist_connections` | bool | `false` | 切换节点时是否中断现有连接 |

## 使用示例

### 示例 1: 基础配置

优先使用香港节点，日本节点作为备选：

```json
{
  "outbounds": [
    {
      "type": "vmess",
      "tag": "hk-node",
      "weight": 2.0,
      "server": "hk.example.com",
      "port": 443,
      "uuid": "xxx"
    },
    {
      "type": "trojan",
      "tag": "jp-node",
      "weight": 1.0,
      "server": "jp.example.com",
      "port": 443,
      "password": "xxx"
    },
    {
      "type": "urltest_pro",
      "tag": "auto",
      "outbounds": ["hk-node", "jp-node"],
      "interval": "5m",
      "tolerance": 50
    }
  ]
}
```

**效果**:
- 香港延迟 100ms，日本延迟 80ms
- 香港分数: 100/2.0 = 50
- 日本分数: 80/1.0 = 80
- 选择香港节点（分数更低）

### 示例 2: 禁用特定节点

临时禁用某个节点而不删除配置：

```json
{
  "outbounds": [
    {
      "type": "ss",
      "tag": "node-a",
      "weight": 1.0,
      "server": "a.example.com"
    },
    {
      "type": "ss",
      "tag": "node-b-disabled",
      "weight": 0,
      "server": "b.example.com"
    },
    {
      "type": "urltest_pro",
      "tag": "auto",
      "outbounds": ["node-a", "node-b-disabled"]
    }
  ]
}
```

**效果**: `node-b-disabled` 不参与选择，即使延迟最低也不会被使用。

### 示例 3: 多级权重分配

按地区设置不同优先级：

```json
{
  "outbounds": [
    {"type": "vmess", "tag": "hk-1", "weight": 3.0, "server": "hk1.example.com"},
    {"type": "vmess", "tag": "hk-2", "weight": 3.0, "server": "hk2.example.com"},
    {"type": "trojan", "tag": "jp-1", "weight": 2.0, "server": "jp1.example.com"},
    {"type": "trojan", "tag": "sg-1", "weight": 1.5, "server": "sg1.example.com"},
    {"type": "ss", "tag": "us-1", "weight": 1.0, "server": "us1.example.com"},
    {
      "type": "urltest_pro",
      "tag": "auto",
      "outbounds": ["hk-1", "hk-2", "jp-1", "sg-1", "us-1"],
      "tolerance": 30
    }
  ]
}
```

**优先级**: 香港 > 日本 > 新加坡 > 美国

### 示例 4: 结合 selector 使用

```json
{
  "outbounds": [
    {"type": "vmess", "tag": "hk", "weight": 2.0, "server": "..."},
    {"type": "vmess", "tag": "jp", "weight": 1.5, "server": "..."},
    {"type": "vmess", "tag": "us", "weight": 1.0, "server": "..."},
    {
      "type": "urltest_pro",
      "tag": "auto-weighted",
      "outbounds": ["hk", "jp", "us"]
    },
    {
      "type": "urltest",
      "tag": "auto-fastest",
      "outbounds": ["hk", "jp", "us"]
    },
    {
      "type": "selector",
      "tag": "proxy",
      "outbounds": ["auto-weighted", "auto-fastest", "hk", "jp", "us"],
      "default": "auto-weighted"
    }
  ]
}
```

## 容差机制说明

`tolerance` 参数用于避免频繁切换节点：

```
当前节点分数: 50
新节点分数: 45
容差值: 10

判断: 50 > 45 + 10 ?
      50 > 55 ? 否

结果: 不切换（差异在容差范围内）
```

只有当新节点分数比当前节点分数低超过容差值时才会切换。

## 与 urltest 的区别

| 特性 | urltest | urltest_pro |
|------|---------|-------------|
| 选择依据 | 纯延迟 | 延迟/权重 |
| 权重支持 | 无 | 支持 |
| 禁用节点 | 需移除配置 | 设置 weight=0 |
| 优先级控制 | 无 | 通过权重调节 |
| 配置兼容 | - | 完全兼容 urltest 参数 |

## 最佳实践

1. **权重设置建议**
   - 优质线路: 2.0 - 3.0
   - 普通线路: 1.0 (默认)
   - 备用线路: 0.5 - 0.8
   - 临时禁用: 0

2. **容差值建议**
   - 稳定优先: 50-100ms
   - 速度优先: 10-30ms
   - 默认推荐: 50ms

3. **测速间隔建议**
   - 移动网络: 1-3 分钟
   - 稳定网络: 3-5 分钟
   - 服务器场景: 5-10 分钟

## 注意事项

- `weight` 字段对所有 outbound 类型生效
- 未配置 `weight` 的节点默认权重为 1.0
- `urltest_pro` 完全兼容 `urltest` 的所有参数
- 权重为 0 的节点会被完全跳过，不参与测速

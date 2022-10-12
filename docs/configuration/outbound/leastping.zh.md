`leastping` 根据最近检查的往返时间平均值来确定节点的响应速度。该策略选择的节点往往是离检查目标服务器较近的节点。

### 结构

```json
{
  "type": "leastping",
  "tag": "leastping-balancer",

  "outbounds": [
    "proxy-"
  ],
  "fallback": "block",
  "health_check": {
    ... // 健康检查字段
  },
  "pick": {
    ... // 负载均衡节点筛选字段
  }
}
```

### 字段

#### outbounds

用于选择的出站标签或标签前缀。例如：若系统中存在 `proxy-a`, `proxy-b`,`proxy-c`:

- `proxy-a` 将匹配特定的 `proxy-a` 出站。
- `proxy-` 将匹配以上所有出站。

#### fallback

==必填==

如果没有符合负载均衡策略的节点，回退到此出站。

### 健康检查字段

参阅 [健康检查](/zh/configuration/shared/health_check/)。

### 负载均衡节点筛选字段

参阅 [负载均衡节点筛选](/zh/configuration/shared/node_pick/)。
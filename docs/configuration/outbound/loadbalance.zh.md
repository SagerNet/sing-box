### 结构

```json
{
  "type": "loadbalance",
  "tag": "auto",

  "outbounds": ["proxy-a", "proxy-b", "proxy-c"],
  "strategy": "round-robin",
  "url": "",
  "interval": "",
  "idle_timeout": ""
}
```

### 字段

#### outbounds

==必填==

用于负载均衡的出站标签列表。

#### strategy

负载均衡策略

| 模式                | 说明                     |
| :------------------ | :------------------------ |
| `round-robin`       | 按顺序循环每个出站          |
| `least-connections` | 选择活跃连接最少的出站      |
| `source-hash`       | 根据客户端 IP 哈希选择      |
| `consistent-hash`   | 一致性哈希环                |

默认使用 `round-robin`。

#### url

用于测试的链接。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

测试间隔。默认使用 `3m`。

#### idle_timeout

空闲超时。默认使用 `30m`。

`dns` 出站是一个内部 DNS 服务器。

### 结构

```json
{
  "outbounds": [
    {
      "type": "dns",
      "tag": "dns-out"
    }
  ]
}
```

!!! note ""

    DNS 出站没有出站连接，所有请求均在内部处理。

### 字段

无字段。
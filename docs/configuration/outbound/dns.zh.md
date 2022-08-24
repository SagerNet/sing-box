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

    There are no outbound connections by the DNS outbound, all requests are handled internally.

### Fields

No fields.
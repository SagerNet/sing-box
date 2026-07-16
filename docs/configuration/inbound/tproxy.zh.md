!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [udp_mapping](/zh/configuration/shared/udp-nat/#udp_mapping)  
    :material-plus: [udp_filtering](/zh/configuration/shared/udp-nat/#udp_filtering)  
    :material-plus: [udp_nat_max](/zh/configuration/shared/udp-nat/#udp_nat_max)

!!! quote ""

    仅支持 Linux。

### 结构

```json
{
  "type": "tproxy",
  "tag": "tproxy-in",

  ... // 监听字段

  "network": "udp",

  ... // UDP NAT 字段
}
```

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### network

监听的网络协议，`tcp` `udp` 之一。

默认所有。

### UDP NAT 字段

参阅 [UDP NAT 字段](/zh/configuration/shared/udp-nat/)。

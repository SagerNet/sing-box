---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "initial_packet_size": 0,
  "disable_path_mtu_discovery": false,

  ... // HTTP2 字段
}
```

### 字段

#### initial_packet_size

初始 QUIC 数据包大小。

#### disable_path_mtu_discovery

禁用 QUIC 路径 MTU 发现。

### HTTP2 字段

参阅 [HTTP2 字段](/zh/configuration/shared/http2/) 了解详情。

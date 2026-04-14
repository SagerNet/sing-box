---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "idle_timeout": "",
  "keep_alive_period": "",
  "stream_receive_window": "",
  "connection_receive_window": "",
  "max_concurrent_streams": 0
}
```

### 字段

#### idle_timeout

空闲连接超时，采用 golang 的 Duration 格式。

#### keep_alive_period

Keep alive 周期，采用 golang 的 Duration 格式。

#### stream_receive_window

HTTP2 流级别流控接收窗口大小。

接受内存大小格式，例如 `"64 MB"`。

#### connection_receive_window

HTTP2 连接级别流控接收窗口大小。

接受内存大小格式，例如 `"64 MB"`。

#### max_concurrent_streams

每个连接的最大并发流数。

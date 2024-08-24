
### Structure

```json
{
  "listen": ":8080",
  "path": "/metrics"
}
```

### Fields

#### listen

Prometheus 指标监听地址，如果为空则禁用。

#### path

HTTP 路径，如果为空则使用 `/metrics`。本路径可用于 Prometheus exporter 进行抓取。

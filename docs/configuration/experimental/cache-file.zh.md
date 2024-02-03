!!! question "自 sing-box 1.8.0 起"

### 结构

```json
{
  "enabled": true,
  "path": "",
  "cache_id": "",
  "store_fakeip": false
}
```

### 字段

#### enabled

启用缓存文件。

#### path

缓存文件路径，默认使用`cache.db`。

#### cache_id

缓存文件中的标识符。

如果不为空，配置特定的数据将使用由其键控的单独存储。

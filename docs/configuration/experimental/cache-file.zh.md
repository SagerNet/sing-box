!!! question "自 sing-box 1.8.0 起"

!!! quote "sing-box 1.14.0 中的更改"

    :material-delete-clock: [store_rdrc](#store_rdrc)  
    :material-plus: [store_dns](#store_dns)

!!! quote "sing-box 1.9.0 中的更改"

    :material-plus: [store_rdrc](#store_rdrc)  
    :material-plus: [rdrc_timeout](#rdrc_timeout)

### 结构

```json
{
  "enabled": true,
  "path": "",
  "cache_id": "",
  "store_fakeip": false,
  "store_rdrc": false,
  "rdrc_timeout": "",
  "store_dns": false
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

#### store_fakeip

将 fakeip 存储在缓存文件中。

#### store_rdrc

!!! failure "已在 sing-box 1.14.0 废弃"

    `store_rdrc` 已在 sing-box 1.14.0 废弃，且将在 sing-box 1.16.0 中被移除，参阅[迁移指南](/zh/migration/#迁移-store_rdrc)。

将拒绝的 DNS 响应缓存存储在缓存文件中。

[旧版地址筛选字段](/zh/configuration/dns/rule/#旧版地址筛选字段) 的检查结果将被缓存至过期。

#### rdrc_timeout

拒绝的 DNS 响应缓存超时。

默认使用 `7d`。

#### store_dns

!!! question "自 sing-box 1.14.0 起"

将 DNS 缓存存储在缓存文件中。

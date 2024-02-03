# 实验性

!!! quote "sing-box 1.8.0 中的更改"

    :material-plus: [cache_file](#cache_file)  
    :material-alert-decagram: [clash_api](#clash_api)

### 结构

```json
{
  "experimental": {
    "cache_file": {},
    "clash_api": {},
    "v2ray_api": {}
  }
}
```

### 字段

| 键            | 格式                       |
|--------------|--------------------------|
| `cache_file` | [缓存文件](./cache-file/)     |
| `clash_api`  | [Clash API](./clash-api/) |
| `v2ray_api`  | [V2Ray API](./v2ray-api/) |
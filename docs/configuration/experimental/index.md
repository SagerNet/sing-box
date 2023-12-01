---
icon: material/alert-decagram
---

# Experimental

!!! quote "Changes in sing-box 1.8.0"

    :material-plus: [cache_file](#cache_file)  
    :material-alert-decagram: [clash_api](#clash_api)

### Structure

```json
{
  "experimental": {
    "cache_file": {},
    "clash_api": {},
    "v2ray_api": {}
  }
}
```

### Fields

| Key          | Format                     |
|--------------|----------------------------|
| `cache_file` | [Cache File](./cache-file) |
| `clash_api`  | [Clash API](./clash-api)   |
| `v2ray_api`  | [V2Ray API](./v2ray-api)   |
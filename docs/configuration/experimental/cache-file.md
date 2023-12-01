---
icon: material/new-box
---

!!! question "Since sing-box 1.8.0"

### Structure

```json
{
  "enabled": true,
  "path": "",
  "cache_id": "",
  "store_fakeip": false
}
```

### Fields

#### enabled

Enable cache file.

#### path

Path to the cache file.

`cache.db` will be used if empty.

#### cache_id

Identifier in cache file.

If not empty, configuration specified data will use a separate store keyed by it.

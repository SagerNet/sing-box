---
icon: material/note-remove
---

!!! failure "已在 sing-box 1.12.0 中被移除"

    GeoIP 已在 sing-box 1.8.0 废弃且在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/zh/migration/#geoip)。

### 结构

```json
{
  "route": {
    "geoip": {
      "path": "",
      "download_url": "",
      "download_detour": ""
    }
  }
}
```

### 字段

#### path

指定 GeoIP 资源的路径。

默认 `geoip.db`。

#### download_url

指定 GeoIP 资源的下载链接。

默认为 `https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db`。

#### download_detour

用于下载 GeoIP 资源的出站的标签。

如果为空，将使用默认出站。
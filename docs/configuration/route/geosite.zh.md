---
icon: material/note-remove
---

!!! failure "已在 sing-box 1.12.0 中被移除"

    Geosite 已在 sing-box 1.8.0 废弃且在 sing-box 1.12.0 中被移除，参阅 [迁移指南](/zh/migration/#geosite)。

### 结构

```json
{
  "route": {
    "geosite": {
      "path": "",
      "download_url": "",
      "download_detour": ""
    }
  }
}
```

### 字段

#### path

指定 GeoSite 资源的路径。

默认 `geosite.db`。

#### download_url

指定 GeoSite 资源的下载链接。

默认为 `https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db`。

#### download_detour

用于下载 GeoSite 资源的出站的标签。

如果为空，将使用默认出站。
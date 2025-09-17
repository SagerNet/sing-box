---
icon: material/new-box
---

!!! question "自 sing-box 1.12.0 起"

# 证书

### 结构

```json
{
  "store": "",
  "certificate": [],
  "certificate_path": [],
  "certificate_directory_path": []
}
```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### 字段

#### store

默认的 X509 受信任 CA 证书列表。

| 类型                | 描述                                                                                       |
|--------------------|--------------------------------------------------------------------------------------------|
| `system`（默认）    | 系统受信任的 CA 证书                                                                        |
| `mozilla`          | [Mozilla 包含列表](https://wiki.mozilla.org/CA/Included_Certificates)（已移除中国 CA 证书） |
| `none`             | 空列表                                                                                     |

#### certificate

要信任的证书行数组，PEM 格式。

#### certificate_path

!!! note ""

    文件修改时将自动重新加载。

要信任的证书路径，PEM 格式。

#### certificate_directory_path

!!! note ""

    文件修改时将自动重新加载。

搜索要信任的证书的目录路径，PEM 格式。
---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

### 结构

```json
{
  "type": "",

  ... // 提供者字段
}
```

### 提供者字段

#### ACME

```json
{
  "type": "acme",
  "service": ""
}
```

##### service

==必填==

要使用的 [ACME 服务](/zh/configuration/service/acme/) 的标签。

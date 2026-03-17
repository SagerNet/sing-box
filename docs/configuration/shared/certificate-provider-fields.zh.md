---
icon: material/new-box
---

!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [certificate_provider](#certificate_provider)

### 结构

```json
{
  "certificate_provider": "my-cert"
}
```

或

```json
{
  "certificate_provider": {
    "type": "acme",

    ... // 提供者字段
  }
}
```

### 字段

#### certificate_provider

字符串或对象。

为字符串时，共享[证书提供者](/zh/configuration/shared/certificate-provider/)的标签。

为对象时，内联的证书提供者。可用类型和字段参阅[证书提供者](/zh/configuration/shared/certificate-provider/)。

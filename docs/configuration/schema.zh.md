---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# JSON Schema

sing-box 为配置文件提供 JSON Schema Draft 2020-12。
兼容的编辑器可使用它提供补全和校验。

### 结构

```json
{
  "$schema": "https://sing-box.sagernet.org/schema.json"
}
```

### 字段

#### $schema

兼容编辑器使用的 Schema URI。
该字段不影响 sing-box 的运行行为。

随本文档发布的 Schema 位于
[sing-box.sagernet.org/schema.json](https://sing-box.sagernet.org/schema.json)。

### 生成

使用以下命令生成与已安装的二进制文件匹配的 Schema：

```bash
sing-box schema -o schema.json
```

未指定 `--output` 时，Schema 将写入标准输出。
生成的 Schema 会反映当前构建中包含的功能。

之后可从配置文件中引用本地 Schema：

```json
{
  "$schema": "./schema.json"
}
```

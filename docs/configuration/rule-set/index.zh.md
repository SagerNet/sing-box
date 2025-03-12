!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: `type: inline`

# 规则集

!!! question "自 sing-box 1.8.0 起"

### 结构

=== "内联"

    !!! question "自 sing-box 1.10.0 起"

    ```json
    {
      "type": "inline", // 可选
      "tag": "",
      "rules": []
    }
    ```

=== "本地文件"

    ```json
    {
      "type": "local",
      "tag": "",
      "format": "source", // or binary
      "path": ""
    }
    ```

=== "远程文件"

    !!! info ""
    
        远程规则集将被缓存如果 `experimental.cache_file.enabled` 已启用。

    ```json
    {
      "type": "remote",
      "tag": "",
      "format": "source", // or binary
      "url": "",
      "update_interval": "", // 可选
      "download_detour": "", // 可选
      "detour": "", // 可选
      "domain_resolver": "" // 可选

      ... // 拨号字段
    }
    ```

### 字段

#### type

==必填==

规则集类型， `local` 或 `remote`。

#### tag

==必填==

规则集的标签。

### 内联字段

!!! question "自 sing-box 1.10.0 起"

#### rules

==必填==

一组 [无头规则](./headless-rule/).

### 本地或远程字段

#### format

==必填==

规则集格式， `source` 或 `binary`。

### 本地字段

#### path

==必填==

!!! note ""

    自 sing-box 1.10.0 起，文件更改时将自动重新加载。

规则集的文件路径。

### 远程字段

#### url

==必填==

规则集的下载 URL。

#### update_interval

规则集的更新间隔。

默认使用 `1d`。

### download_detour
保留此字段只是为了保证兼容性，请使用detour字段
在此字段和detour字段的内容都有效时，优先使用detour字段的内容

#### detour

用于下载规则集的出站的标签。

如果为空，将使用默认出站。

#### domain_resolver

用于设置解析域名的域名解析器。

如果此选项和router.default_domain_resolver同时设置，router.default_domain_resolver会被覆盖

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。

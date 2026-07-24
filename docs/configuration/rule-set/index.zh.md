!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [http_client](#http_client)  
    :material-delete-clock: [download_detour](#download_detour)  
    :material-alert: [tag](#tag)  
    :material-plus: [initial_path](#initial_path)

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
      "tag": "", // 或 []
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
      "tag": "", // 或 []
      "format": "source", // or binary
      "url": "",
      "initial_path": "",
      "http_client": "", // 或 {}
      "update_interval": "",

      // 废弃的

      "download_detour": ""
    }
    ```

### 字段

#### type

==必填==

规则集类型， `local` 或 `remote`。

#### tag

==必填==

规则集的标签。

!!! question "自 sing-box 1.14.0 起"

    `tag` 也接受一组标签，用于一次定义多个共享其他选项的规则集。

    `path`、`url` 或 `initial_path` 中的 `{tag}` 占位符将被替换为每个标签，设置多个标签时必填。

    多个标签与 `type: inline` 冲突。

### 内联字段

!!! question "自 sing-box 1.10.0 起"

#### rules

==必填==

一组 [无头规则](./headless-rule/).

### 本地或远程字段

#### format

==必填==

规则集格式， `source` 或 `binary`。

当 `path` 或 `url` 使用 `json` 或 `srs` 作为扩展名时可选。

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

#### initial_path

!!! question "自 sing-box 1.14.0 起"

规则集初始内容的文件路径。

仅在启动时没有可用的规则集缓存时读取一次，使启动不被初始下载阻塞。
启动后规则集仍会立即在后台更新。

#### http_client

!!! question "自 sing-box 1.14.0 起"

用于下载规则集的 HTTP 客户端。

参阅 [HTTP 客户端字段](/zh/configuration/shared/http-client/) 了解详情。

留空时使用默认 HTTP 客户端：即由 [`default_http_client`](/zh/configuration/route/#default_http_client)
指定的客户端，或当 `default_http_client` 为空时使用顶级 `http_clients` 的第一项。

!!! failure "隐式默认已在 sing-box 1.14.0 废弃"

    当 `http_clients` 与 `default_http_client` 均未配置时，将使用通过默认出站连接的隐式 HTTP 客户端。
    该隐式默认已在 sing-box 1.14.0 废弃，并将在 sing-box 1.16.0 移除；请改为定义 `http_clients`。

#### update_interval

规则集的更新间隔。

默认使用 `1d`。

#### download_detour

!!! failure "已在 sing-box 1.14.0 废弃"

    `download_detour` 已在 sing-box 1.14.0 废弃且将在 sing-box 1.16.0 中被移除，请使用 `http_client` 代替。

用于下载规则集的出站的标签。

!!! quote "sing-box 1.10.0 中的更改"

    :material-plus: [access_control_allow_origin](#access_control_allow_origin)  
    :material-plus: [access_control_allow_private_network](#access_control_allow_private_network)

!!! quote "sing-box 1.8.0 中的更改"

    :material-delete-alert: [store_mode](#store_mode)  
    :material-delete-alert: [store_selected](#store_selected)  
    :material-delete-alert: [store_fakeip](#store_fakeip)  
    :material-delete-alert: [cache_file](#cache_file)  
    :material-delete-alert: [cache_id](#cache_id)

### 结构

=== "结构"

    ```json
    {
      "external_controller": "127.0.0.1:9090",
      "external_ui": "",
      "external_ui_download_url": "",
      "external_ui_download_detour": "",
      "secret": "",
      "default_mode": "",
      "access_control_allow_origin": [],
      "access_control_allow_private_network": false,
      
      // Deprecated
      
      "store_mode": false,
      "store_selected": false,
      "store_fakeip": false,
      "cache_file": "",
      "cache_id": ""
    }
    ```

=== "示例 (在线)"

    !!! question "自 sing-box 1.10.0 起"

    ```json
    {
      "external_controller": "127.0.0.1:9090",
      "access_control_allow_origin": [
        "http://127.0.0.1",
        "http://yacd.haishan.me"
      ],
      "access_control_allow_private_network": true
    }
    ```

=== "示例 (下载)"

    !!! question "自 sing-box 1.10.0 起"

    ```json
    {
      "external_controller": "0.0.0.0:9090",
      "external_ui": "dashboard"
      // "external_ui_download_detour": "direct"
    }
    ```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签

### Fields

#### external_controller

RESTful web API 监听地址。如果为空，则禁用 Clash API。

#### external_ui

到静态网页资源目录的相对路径或绝对路径。sing-box 会在 `http://{{external-controller}}/ui` 下提供它。

#### external_ui_download_url

静态网页资源的 ZIP 下载 URL，如果指定的 `external_ui` 目录为空，将使用。

默认使用 `https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip`。

#### external_ui_download_detour

用于下载静态网页资源的出站的标签。

如果为空，将使用默认出站。

#### secret

RESTful API 的密钥（可选）
通过指定 HTTP 标头 `Authorization: Bearer ${secret}` 进行身份验证
如果 RESTful API 正在监听 0.0.0.0，请始终设置一个密钥。

#### default_mode

Clash 中的默认模式，默认使用 `Rule`。

此设置没有直接影响，但可以通过 `clash_mode` 规则项在路由和 DNS 规则中使用。

#### access_control_allow_origin

!!! question "自 sing-box 1.10.0 起"

允许的 CORS 来源，默认使用 `*`。

要从公共网站访问私有网络上的 Clash API，必须在 `access_control_allow_origin` 中明确指定它而不是使用 `*`。

#### access_control_allow_private_network

!!! question "自 sing-box 1.10.0 起"

允许从私有网络访问。

要从公共网站访问私有网络上的 Clash API，必须启用 `access_control_allow_private_network`。

#### store_mode

!!! failure "已在 sing-box 1.8.0 废弃"

    `store_mode` 已在 Clash API 中废弃，且默认启用当 `cache_file.enabled`。

将 Clash 模式存储在缓存文件中。

#### store_selected

!!! failure "已在 sing-box 1.8.0 废弃"

    `store_selected` 已在 Clash API 中废弃，且默认启用当 `cache_file.enabled`。

!!! note ""

    必须为目标出站设置标签。

将 `Selector` 中出站的选定的目标出站存储在缓存文件中。

#### store_fakeip

!!! failure "已在 sing-box 1.8.0 废弃"

    `store_selected` 已在 Clash API 中废弃，且已迁移到 `cache_file.store_fakeip`。

将 fakeip 存储在缓存文件中。

#### cache_file

!!! failure "已在 sing-box 1.8.0 废弃"
 
    `cache_file` 已在 Clash API 中废弃，且已迁移到 `cache_file.enabled` 和 `cache_file.path`。

缓存文件路径，默认使用`cache.db`。

#### cache_id

!!! failure "已在 sing-box 1.8.0 废弃"
 
    `cache_id` 已在 Clash API 中废弃，且已迁移到 `cache_file.cache_id`。

缓存 ID。

如果不为空，配置特定的数据将使用由其键控的单独存储。

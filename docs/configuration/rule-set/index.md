!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [http_client](#http_client)  
    :material-delete-clock: [download_detour](#download_detour)  
    :material-alert: [tag](#tag)  
    :material-plus: [initial_path](#initial_path)

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: `type: inline`

# rule-set

!!! question "Since sing-box 1.8.0"

### Structure

=== "Inline"

    !!! question "Since sing-box 1.10.0"

    ```json
    {
      "type": "inline", // optional
      "tag": "",
      "rules": []
    }
    ```

=== "Local File"

    ```json
    {
      "type": "local",
      "tag": "", // or []
      "format": "source", // or binary
      "path": ""
    }
    ```

=== "Remote File"

    !!! info ""
    
        Remote rule-set will be cached if `experimental.cache_file.enabled`.

    ```json
    {
      "type": "remote",
      "tag": "", // or []
      "format": "source", // or binary
      "url": "",
      "initial_path": "",
      "http_client": "", // or {}
      "update_interval": "",

      // Deprecated

      "download_detour": ""
    }
    ```

### Fields

#### type

==Required==

Type of rule-set, `local` or `remote`.

#### tag

==Required==

Tag of rule-set.

!!! question "Since sing-box 1.14.0"

    `tag` also accepts a list of tags to define multiple rule-sets sharing other options at once.

    The `{tag}` placeholder in `path`, `url` or `initial_path` is replaced by each tag,
    and is required when multiple tags are set.

    Multiple tags conflict with `type: inline`.

### Inline Fields

!!! question "Since sing-box 1.10.0"

#### rules

==Required==

List of [Headless Rule](./headless-rule/).

### Local or Remote Fields

#### format

==Required==

Format of rule-set file, `source` or `binary`.

Optional when `path` or `url` uses `json` or `srs` as extension.

### Local Fields

#### path

==Required==

!!! note ""

    Will be automatically reloaded if file modified since sing-box 1.10.0.

File path of rule-set.

### Remote Fields

#### url

==Required==

Download URL of rule-set.

#### initial_path

!!! question "Since sing-box 1.14.0"

File path of the initial rule-set content.

Read once at startup when no cached rule-set is available, so startup is not
blocked by the initial download. The rule-set is still updated in the background
immediately after startup.

#### http_client

!!! question "Since sing-box 1.14.0"

HTTP Client for downloading rule-set.

See [HTTP Client Fields](/configuration/shared/http-client/) for details.

When empty, the default HTTP client is used: the one named by
[`default_http_client`](/configuration/route/#default_http_client), or the first top-level
`http_clients` entry when `default_http_client` is empty.

!!! failure "Implicit default deprecated in sing-box 1.14.0"

    When neither `http_clients` nor `default_http_client` is configured, an implicit HTTP
    client connecting through the default outbound is used. This implicit default is
    deprecated in sing-box 1.14.0 and will be removed in sing-box 1.16.0; define
    `http_clients` instead.

#### update_interval

Update interval of rule-set.

`1d` will be used if empty.

#### download_detour

!!! failure "Deprecated in sing-box 1.14.0"

    `download_detour` is deprecated in sing-box 1.14.0 and will be removed in sing-box 1.16.0, use `http_client` instead.

Tag of the outbound to download rule-set.

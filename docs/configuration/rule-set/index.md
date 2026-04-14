!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [http_client](#http_client)  
    :material-delete-clock: [download_detour](#download_detour)

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
      "tag": "",
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
      "tag": "",
      "format": "source", // or binary
      "url": "",
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

#### http_client

!!! question "Since sing-box 1.14.0"

HTTP Client for downloading rule-set.

See [HTTP Client Fields](/configuration/shared/http-client/) for details.

Default transport will be used if empty.

#### update_interval

Update interval of rule-set.

`1d` will be used if empty.

#### download_detour

!!! failure "Deprecated in sing-box 1.14.0"

    `download_detour` is deprecated in sing-box 1.14.0 and will be removed in sing-box 1.16.0, use `http_client` instead.

Tag of the outbound to download rule-set.

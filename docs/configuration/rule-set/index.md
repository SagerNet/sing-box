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
      "download_detour": "", // optional
      "update_interval": "" // optional
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

#### download_detour

Tag of the outbound to download rule-set.

Default outbound will be used if empty.

#### update_interval

Update interval of rule-set.

`1d` will be used if empty.

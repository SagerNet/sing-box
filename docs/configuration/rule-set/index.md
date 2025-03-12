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
      "update_interval": "", // optional
      "download_detour": "", // optional
      "detour": "", // optional
      "domain_resolver": "" // optional

      ... // Dial Fields
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

#### update_interval

Update interval of rule-set.

`1d` will be used if empty.

### download_detour
This field is retained for compatibility only, please use the detour field.
When both this field and the detour field have valid content, the content of the detour field takes precedence.

#### detour

Tag of the outbound to download rule-set.

Default outbound will be used if empty.

#### domain_resolver

Set domain resolver to use for resolving domain names.

If this option and router.default_domain_resolver are set at the same time, router.default_domain_resolver will be overwritten

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
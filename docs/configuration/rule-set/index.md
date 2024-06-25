# rule-set

!!! question "Since sing-box 1.8.0"

### Structure

```json
{
  "type": "",
  "tag": "",
  "format": "",
  
  ... // Typed Fields
}
```

#### Local Structure

```json
{
  "type": "local",
  
  ...
  
  "path": ""
}
```

#### Remote Structure

!!! info ""

    Remote rule-set will be cached if `experimental.cache_file.enabled`.

```json
{
  "type": "remote",
  
  ...,
  
  "url": "",
  "download_detour": "",
  "update_interval": ""
}
```

### Fields

#### type

==Required==

Type of rule-set, `local` or `remote`.

#### tag

==Required==

Tag of rule-set.

#### format

==Required==

Format of rule-set, `source` or `binary`.

### Local Fields

#### path

==Required==

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

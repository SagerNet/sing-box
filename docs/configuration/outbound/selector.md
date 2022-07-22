### Structure

```json
{
  "outbounds": [
    {
      "type": "selector",
      "tag": "select",
      
      "outbounds": [
        "proxy-a",
        "proxy-b",
        "proxy-c"
      ],
      "default": "proxy-c"
    }
  ]
}
```

!!! error ""

    The selector can only be controlled through the [Clash API](/configuration/experimental#clash-api-fields) currently.

### Fields

#### outbounds

==Required==

List of outbound tags to select.

#### default

The default outbound tag. The first outbound will be used if empty.
### 结构

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

    选择器目前只能通过 [Clash API](/zh/configuration/experimental#clash-api) 来控制。

### 字段

#### outbounds

==必填==

用于选择的出站标签列表。

#### default

默认的出站标签。如果为空，则使用第一个出站。

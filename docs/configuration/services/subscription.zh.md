# 订阅

### 结构

```json
{
  "type": "subscription",
  "tag": "all.sub",

  "interval": "1h",
  "providers": [
    {
      "tag": "provider1",
      "url": "https://url.to/subscription",
      "exclude": "",
      "include": ""
    }
  ]
}
```

### 字段

#### interval

刷新订阅的时间间隔。最小值为 `1m`，默认值为 `1h`。

#### providers

订阅源列表。

#### providers.tag

==必填==

订阅源的标签。

所设订阅源中有一个节点 `node1`，则该节点导入后的标签为 `all.sub.provider1.node1`。

#### providers.url

==必填==

订阅源的 URL。

#### providers.exclude

排除节点的正则表达式。排除表达式的优先级高于包含表达式。

#### providers.include

包含节点的正则表达式。

# Subscription

### Structure

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

### Fields

#### interval

Refresh interval of the subscription. The minimum value is `1m`, the default value is `1h`.

#### providers

List of subscription providers.

#### providers.tag

==Required==

Tag of the subscription provider.

Suppose we have `node1` from the subscription provider, the tag of the outbound will be `all.sub.provider1.node1`.

#### providers.url

==Required==

URL of the subscription provider.

#### providers.exclude

Regular expression to exclude nodes. The priority of the exclude expression is higher than the include expression.

#### providers.include

Regular expression to include nodes.

### Structure

```json
{
  "outbounds": [
    {
      "type": "urltest",
      "tag": "auto",
      
      "outbounds": [
        "proxy-a",
        "proxy-b",
        "proxy-c"
      ],
      "url": "http://www.gstatic.com/generate_204",
      "interval": "1m",
      "tolerance": 50
    }
  ]
}
```

### Fields

#### outbounds

==Required==

List of outbound tags to test.

#### url

The URL to test. `http://www.gstatic.com/generate_204` will be used if empty.

#### interval

The test interval. `1m` will be used if empty.

#### tolerance

The test tolerance in milliseconds. `50` will be used if empty.

### Structure

```json
{
  "type": "shadowtls",
  "tag": "st-in",

  ... // Listen Fields

  "handshake": {
    "server": "google.com",
    "server_port": 443,
    
    ... // Dial Fields
  }
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.


### Fields

#### handshake

==Required==

Handshake server address and [dial options](/configuration/shared/dial).


---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# sing-box API

The sing-box API service is a gRPC server for observing and controlling the running sing-box instance.

The server also accepts [gRPC-Web](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md) requests,
including the WebSocket transport of [@improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web)
for bidirectional streaming methods.

### Structure

```json
{
  "type": "api",
  
  ... // Listen Fields
  
  "secret": "",
  "access_control_allow_origin": [],
  "access_control_allow_private_network": false,
  "tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### secret

Secret for the API.

Clients authenticate with the standard `authorization: Bearer <secret>` gRPC metadata header.

If empty, authentication is disabled.

#### access_control_allow_origin

CORS allowed origins, `*` will be used if empty.

#### access_control_allow_private_network

Allow access from private network.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

Connection tracking and Clash mode methods require [Clash API](/configuration/experimental/clash-api/)
to be configured, otherwise they fail with `UNIMPLEMENTED`.

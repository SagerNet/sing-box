---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# sing-box API

The sing-box API service is a gRPC server for observing and controlling the running sing-box instance.

It can be accessed by the [sing-box graphical clients](/clients/) for iOS, macOS, and
Android (via the Remote Control feature), or the
[sing-box dashboard](https://github.com/SagerNet/sing-box-dashboard).

The server also accepts [gRPC-Web](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md) requests.

### Structure

```json
{
  "type": "api",
  
  ... // Listen Fields
  
  "secret": "",
  "access_control_allow_origin": [],
  "access_control_allow_private_network": false,
  "dashboard": {
    "enabled": true,
    "path": "",
    "download_url": "",
    "http_client": "", // or {}
    "update_interval": ""
  },
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

#### dashboard

Web dashboard downloaded and served over the API listener at `/dashboard/`; other browser
requests are redirected to it.

!!! info ""

    The object can be replaced with a boolean value (equivalent to `{ "enabled": <bool> }`),
    or with a string path (equivalent to `{ "enabled": true, "path": "<string>" }`).

##### enabled

Enable the dashboard.

##### path

Directory the dashboard files are stored in.

`dashboard` in the working directory will be used by default.

If the directory contains files not downloaded by sing-box, they are served as-is.

##### download_url

Download URL of the dashboard archive (zip).

`https://github.com/SagerNet/sing-box-dashboard/archive/refs/heads/gh-pages.zip` will be used by default.

##### http_client

HTTP client used to download the dashboard.

See [HTTP Client Fields](/configuration/shared/http-client/) for details.

##### update_interval

Update interval of the dashboard.

`1d` will be used by default.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

---
icon: material/new-box
---

# Cloudflare Origin CA

### Structure

```json
{
  "type": "cloudflare-origin-ca",
  "tag": "",

  "hostnames": [],
  "data_directory": "",
  "api_token": "",
  "origin_ca_key": "",
  "request_type": "",
  "requested_validity": 0,
  "renew_before": "",
  "request_timeout": ""
}
```

This provider generates a private key locally, submits a CSR to Cloudflare Origin CA, and serves the returned
certificate as a TLS certificate provider.

### Fields

#### hostnames

List of hostnames or wildcard hostnames to include in the certificate.

Unicode hostnames are converted to punycode automatically.

#### data_directory

The directory to store the issued certificate and private key.

If empty, the certificate is kept only in memory and will be re-issued after restart.

#### api_token

Cloudflare API token used to create the certificate.

Requires the `Zone / SSL and Certificates / Edit` permission.

Mutually exclusive with `origin_ca_key`.

#### origin_ca_key

Cloudflare Origin CA key used as the `X-Auth-User-Service-Key` header.

Mutually exclusive with `api_token`.

#### request_type

The signature type to request from Cloudflare.

| Value                | Type        |
|----------------------|-------------|
| `origin-rsa`         | RSA         |
| `origin-ecc`         | ECDSA P-256 |

`origin-rsa` is used if empty.

#### requested_validity

The requested certificate validity in days.

Available values: `7`, `30`, `90`, `365`, `730`, `1095`, `5475`.

`5475` is used if empty.

#### renew_before

How long before expiration sing-box should request a replacement certificate.

If empty, the smaller of `30d` and one third of the certificate lifetime is used.

#### request_timeout

HTTP timeout for requests to the Cloudflare API.

`30s` is used if empty.

---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# Cloudflare Origin CA

### Structure

```json
{
  "type": "cloudflare-origin-ca",
  "tag": "",

  "domain": [],
  "data_directory": "",
  "api_token": "",
  "origin_ca_key": "",
  "request_type": "",
  "requested_validity": 0,
  "http_client": "" // or {}
}
```

### Fields

#### domain

==Required==

List of domain names or wildcard domain names to include in the certificate.

#### data_directory

Root directory used to store the issued certificate, private key, and metadata.

If empty, sing-box uses the same default data directory as the ACME certificate provider:
`$XDG_DATA_HOME/certmagic` or `$HOME/.local/share/certmagic`.

#### api_token

Cloudflare API token used to create the certificate.

Get or create one in [Cloudflare Dashboard > My Profile > API Tokens](https://dash.cloudflare.com/profile/api-tokens).

Requires the `Zone / SSL and Certificates / Edit` permission.

Conflict with `origin_ca_key`.

#### origin_ca_key

Cloudflare Origin CA Key.

Get it in [Cloudflare Dashboard > My Profile > API Tokens > API Keys > Origin CA Key](https://dash.cloudflare.com/profile/api-tokens).

Conflict with `api_token`.

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

`5475` days (15 years) is used if empty.

#### http_client

HTTP Client for all provider HTTP requests.

See [HTTP Client Fields](/configuration/shared/http-client/) for details.

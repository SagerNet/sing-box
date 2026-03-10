---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# OCM

OCM (OpenAI Codex Multiplexer) service is a multiplexing service that allows you to access your local OpenAI Codex subscription remotely through custom tokens.

It handles OAuth authentication with OpenAI's API on your local machine while allowing remote clients to authenticate using custom tokens.

### Structure

```json
{
  "type": "ocm",

  ... // Listen Fields

  "credential_path": "",
  "usages_path": "",
  "users": [],
  "headers": {},
  "detour": "",
  "tls": {}
}
```

### Listen Fields

See [Listen Fields](/configuration/shared/listen/) for details.

### Fields

#### credential_path

Path to the OpenAI OAuth credentials file.

If not specified, defaults to:
- `$CODEX_HOME/auth.json` if `CODEX_HOME` environment variable is set
- `~/.codex/auth.json` otherwise

Refreshed tokens are automatically written back to the same location.

#### usages_path

Path to the file for storing aggregated API usage statistics.

Usage tracking is disabled if not specified.

When enabled, the service tracks and saves comprehensive statistics including:
- Request counts
- Token usage (input, output, cached)
- Calculated costs in USD based on OpenAI API pricing

Statistics are organized by model and optionally by user when authentication is enabled.

The statistics file is automatically saved every minute and upon service shutdown.

#### users

List of authorized users for token authentication.

If empty, no authentication is required.

Object format:

```json
{
  "name": "",
  "token": ""
}
```

Object fields:

- `name`: Username identifier for tracking purposes.
- `token`: Bearer token for authentication. Clients authenticate by setting the `Authorization: Bearer <token>` header.

#### headers

Custom HTTP headers to send to the OpenAI API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the OpenAI API.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

### Example

#### Server

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "127.0.0.1",
      "listen_port": 8080
    }
  ]
}
```

#### Client

Add to `~/.codex/config.toml`:

```toml
# profile = "ocm"                # set as default profile

[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
supports_websockets = true

[profiles.ocm]
model_provider = "ocm"
# model = "gpt-5.4"              # if the latest model is not yet publicly released
# model_reasoning_effort = "xhigh"
```

Then run:

```bash
codex --profile ocm
```

### Example with Authentication

#### Server

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "usages_path": "./codex-usages.json",
      "users": [
        {
          "name": "alice",
          "token": "sk-ocm-hello-world"
        },
        {
          "name": "bob",
          "token": "sk-ocm-hello-bob"
        }
      ]
    }
  ]
}
```

#### Client

Add to `~/.codex/config.toml`:

```toml
# profile = "ocm"                # set as default profile

[model_providers.ocm]
name = "OCM Proxy"
base_url = "http://127.0.0.1:8080/v1"
supports_websockets = true
experimental_bearer_token = "sk-ocm-hello-world"

[profiles.ocm]
model_provider = "ocm"
# model = "gpt-5.4"              # if the latest model is not yet publicly released
# model_reasoning_effort = "xhigh"
```

Then run:

```bash
codex --profile ocm
```

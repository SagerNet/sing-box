---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# OCM

OCM (OpenAI Codex Multiplexer) service is a multiplexing service that allows you to access your local OpenAI Codex subscription remotely through custom tokens.

It handles OAuth authentication with OpenAI's API on your local machine while allowing remote clients to authenticate using custom tokens.

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [credentials](#credentials)  
    :material-alert: [users](#users)

### Structure

```json
{
  "type": "ocm",

  ... // Listen Fields

  "credential_path": "",
  "credentials": [],
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

Conflict with `credentials`.

#### credentials

!!! question "Since sing-box 1.14.0"

List of credential configurations for multi-credential mode.

When set, top-level `credential_path`, `usages_path`, and `detour` are forbidden. Each user must specify a `credential` tag.

Each credential has a `type` field (`default`, `balancer`, or `fallback`) and a required `tag` field.

##### Default Credential

```json
{
  "tag": "a",
  "credential_path": "/path/to/auth.json",
  "usages_path": "/path/to/usages.json",
  "detour": "",
  "reserve_5h": 20,
  "reserve_weekly": 20
}
```

A single OAuth credential file. The `type` field can be omitted (defaults to `default`).

- `credential_path`: Path to the credentials file. Same defaults as top-level `credential_path`.
- `usages_path`: Optional usage tracking file for this credential.
- `detour`: Outbound tag for connecting to the OpenAI API with this credential.
- `reserve_5h`: Reserve threshold (1-99) for primary rate limit window. Credential pauses at (100-N)% utilization.
- `reserve_weekly`: Reserve threshold (1-99) for secondary (weekly) rate limit window. Credential pauses at (100-N)% utilization.

##### Balancer Credential

```json
{
  "tag": "pool",
  "type": "balancer",
  "strategy": "",
  "credentials": ["a", "b"],
  "poll_interval": "60s"
}
```

Assigns sessions to default credentials based on the selected strategy. Sessions are sticky until the assigned credential hits a rate limit.

- `strategy`: Selection strategy. One of `least_used` `round_robin` `random`. `least_used` will be used by default.
- `credentials`: ==Required== List of default credential tags.
- `poll_interval`: How often to poll upstream usage API. Default `60s`.

##### Fallback Credential

```json
{
  "tag": "backup",
  "type": "fallback",
  "credentials": ["a", "b"],
  "poll_interval": "30s"
}
```

Uses credentials in order. Falls through to the next when the current one is exhausted.

- `credentials`: ==Required== Ordered list of default credential tags.
- `poll_interval`: How often to poll upstream usage API. Default `60s`.

#### usages_path

Path to the file for storing aggregated API usage statistics.

Usage tracking is disabled if not specified.

When enabled, the service tracks and saves comprehensive statistics including:
- Request counts
- Token usage (input, output, cached)
- Calculated costs in USD based on OpenAI API pricing

Statistics are organized by model and optionally by user when authentication is enabled.

The statistics file is automatically saved every minute and upon service shutdown.

Conflict with `credentials`. In multi-credential mode, use `usages_path` on individual default credentials.

#### users

List of authorized users for token authentication.

If empty, no authentication is required.

Object format:

```json
{
  "name": "",
  "token": "",
  "credential": ""
}
```

Object fields:

- `name`: Username identifier for tracking purposes.
- `token`: Bearer token for authentication. Clients authenticate by setting the `Authorization: Bearer <token>` header.
- `credential`: Credential tag to use for this user. ==Required== when `credentials` is set.

#### headers

Custom HTTP headers to send to the OpenAI API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the OpenAI API.

Conflict with `credentials`. In multi-credential mode, use `detour` on individual default credentials.

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

### Example with Multiple Credentials

#### Server

```json
{
  "services": [
    {
      "type": "ocm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "credentials": [
        {
          "tag": "a",
          "credential_path": "/home/user/.codex-a/auth.json",
          "usages_path": "/data/usages-a.json",
          "reserve_5h": 20,
          "reserve_weekly": 20
        },
        {
          "tag": "b",
          "credential_path": "/home/user/.codex-b/auth.json",
          "reserve_5h": 10,
          "reserve_weekly": 10
        },
        {
          "tag": "pool",
          "type": "balancer",
          "poll_interval": "60s",
          "credentials": ["a", "b"]
        }
      ],
      "users": [
        {
          "name": "alice",
          "token": "sk-ocm-hello-world",
          "credential": "pool"
        },
        {
          "name": "bob",
          "token": "sk-ocm-hello-bob",
          "credential": "a"
        }
      ]
    }
  ]
}
```

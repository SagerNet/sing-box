---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# OCM

OCM (OpenAI Codex Multiplexer) service is a multiplexing service that allows you to access your local OpenAI Codex subscription remotely through custom tokens.

It handles OAuth authentication with OpenAI's API on your local machine while allowing remote clients to authenticate using custom tokens.

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [credentials](#credentials)  
    :material-alert: [credential_path](#credential_path)  
    :material-alert: [usages_path](#usages_path)  
    :material-alert: [users](#users)  
    :material-alert: [detour](#detour)

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

!!! question "Since sing-box 1.14.0"

When `credential_path` points to a file, the service can start before the file exists. The credential becomes available automatically after the file is created or updated, and becomes unavailable immediately if the file is later removed or becomes invalid.

Conflict with `credentials`.

#### credentials

!!! question "Since sing-box 1.14.0"

List of credential configurations for multi-credential mode.

When set, top-level `credential_path`, `usages_path`, and `detour` are forbidden. Each user must specify a `credential` tag.

Each credential has a `type` field (`default`, `external`, or `balancer`) and a required `tag` field.

##### Default Credential

```json
{
  "tag": "a",
  "credential_path": "/path/to/auth.json",
  "usages_path": "/path/to/usages.json",
  "detour": "",
  "reserve_5h": 20,
  "reserve_weekly": 20,
  "limit_5h": 0,
  "limit_weekly": 0
}
```

A single OAuth credential file. The `type` field can be omitted (defaults to `default`). The service can start before the file exists, and reloads file updates automatically.

- `credential_path`: Path to the credentials file. Same defaults as top-level `credential_path`.
- `usages_path`: Optional usage tracking file for this credential.
- `detour`: Outbound tag for connecting to the OpenAI API with this credential.
- `reserve_5h`: Reserve threshold (1-99) for primary rate limit window. Credential pauses at (100-N)% utilization. Conflict with `limit_5h`.
- `reserve_weekly`: Reserve threshold (1-99) for secondary (weekly) rate limit window. Credential pauses at (100-N)% utilization. Conflict with `limit_weekly`.
- `limit_5h`: Explicit utilization cap (0-100) for primary rate limit window. `0` means unset. Credential pauses when utilization reaches this value. Conflict with `reserve_5h`.
- `limit_weekly`: Explicit utilization cap (0-100) for secondary (weekly) rate limit window. `0` means unset. Credential pauses when utilization reaches this value. Conflict with `reserve_weekly`.

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

- `strategy`: Selection strategy. One of `least_used` `round_robin` `random` `fallback`. `least_used` will be used by default.
- `credentials`: ==Required== List of default credential tags.
- `poll_interval`: How often to poll upstream usage API. Default `60s`.

##### Fallback Strategy

```json
{
  "tag": "backup",
  "type": "balancer",
  "strategy": "fallback",
  "credentials": ["a", "b"],
  "poll_interval": "30s"
}
```

A balancer with `strategy: "fallback"` uses credentials in order. It falls through to the next when the current one is exhausted.

- `credentials`: ==Required== Ordered list of default credential tags.
- `poll_interval`: How often to poll upstream usage API. Default `60s`.

##### External Credential

```json
{
  "tag": "remote",
  "type": "external",
  "url": "",
  "server": "",
  "server_port": 0,
  "token": "",
  "reverse": false,
  "detour": "",
  "usages_path": "",
  "poll_interval": "30m"
}
```

Proxies requests through a remote OCM instance instead of using a local OAuth credential.

- `url`: URL of the remote OCM instance. Omit to create a receiver that only waits for inbound reverse connections.
- `server`: Override server address for dialing, separate from URL hostname.
- `server_port`: Override server port for dialing.
- `token`: ==Required== Authentication token for the remote instance.
- `reverse`: Enable connector mode. Requires `url`. A connector dials out to `/ocm/v1/reverse` on the remote instance and cannot serve local requests directly. When `url` is set without `reverse`, the credential proxies requests through the remote instance normally and prefers an established reverse connection when one is available.
- `detour`: Outbound tag for connecting to the remote instance.
- `usages_path`: Optional usage tracking file.
- `poll_interval`: How often to poll the remote status endpoint. Default `30m`.

#### usages_path

Path to the file for storing aggregated API usage statistics.

Usage tracking is disabled if not specified.

When enabled, the service tracks and saves comprehensive statistics including:
- Request counts
- Token usage (input, output, cached)
- Calculated costs in USD based on OpenAI API pricing

Statistics are organized by model and optionally by user when authentication is enabled.

The statistics file is automatically saved every minute and upon service shutdown.

!!! question "Since sing-box 1.14.0"

Conflict with `credentials`. In multi-credential mode, use `usages_path` on individual default credentials.

#### users

List of authorized users for token authentication.

If empty, no authentication is required.

Object format:

```json
{
  "name": "",
  "token": "",
  "credential": "",
  "external_credential": "",
  "allow_external_usage": false
}
```

Object fields:

- `name`: Username identifier for tracking purposes.
- `token`: Bearer token for authentication. Clients authenticate by setting the `Authorization: Bearer <token>` header.

!!! question "Since sing-box 1.14.0"

- `credential`: Credential tag to use for this user. ==Required== when `credentials` is set.
- `external_credential`: Tag of an external credential used only to rewrite response rate-limit headers with aggregated utilization from this user's other available credentials. It does not control request routing; request selection still comes from `credential` and `allow_external_usage`.
- `allow_external_usage`: Allow this user to use external credentials. `false` by default.

#### headers

Custom HTTP headers to send to the OpenAI API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the OpenAI API.

!!! question "Since sing-box 1.14.0"

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

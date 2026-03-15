---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# CCM

CCM (Claude Code Multiplexer) service is a multiplexing service that allows you to access your local Claude Code subscription remotely through custom tokens.

It handles OAuth authentication with Claude's API on your local machine while allowing remote Claude Code to authenticate using Auth Tokens via the `ANTHROPIC_AUTH_TOKEN` environment variable.

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [credentials](#credentials)  
    :material-alert: [credential_path](#credential_path)  
    :material-alert: [usages_path](#usages_path)  
    :material-alert: [users](#users)  
    :material-alert: [detour](#detour)

### Structure

```json
{
  "type": "ccm",

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

Path to the Claude Code OAuth credentials file.

If not specified, defaults to:
- `$CLAUDE_CONFIG_DIR/.credentials.json` if `CLAUDE_CONFIG_DIR` environment variable is set
- `~/.claude/.credentials.json` otherwise

On macOS, credentials are read from the system keychain first, then fall back to the file if unavailable.

Refreshed tokens are automatically written back to the same location.

!!! question "Since sing-box 1.14.0"

When `credential_path` points to a file, the service can start before the file exists. The credential becomes available automatically after the file is created or updated, and becomes unavailable immediately if the file is later removed or becomes invalid.

On macOS without an explicit `credential_path`, keychain changes are not watched. Automatic reload only applies to the credential file path.

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
  "credential_path": "/path/to/.credentials.json",
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
- `detour`: Outbound tag for connecting to the Claude API with this credential.
- `reserve_5h`: Reserve threshold (1-99) for 5-hour window. Credential pauses at (100-N)% utilization. Conflict with `limit_5h`.
- `reserve_weekly`: Reserve threshold (1-99) for weekly window. Credential pauses at (100-N)% utilization. Conflict with `limit_weekly`.
- `limit_5h`: Explicit utilization cap (0-100) for 5-hour window. `0` means unset. Credential pauses when utilization reaches this value. Conflict with `reserve_5h`.
- `limit_weekly`: Explicit utilization cap (0-100) for weekly window. `0` means unset. Credential pauses when utilization reaches this value. Conflict with `reserve_weekly`.

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

Proxies requests through a remote CCM instance instead of using a local OAuth credential.

- `url`: URL of the remote CCM instance. Omit to create a receiver that only waits for inbound reverse connections.
- `server`: Override server address for dialing, separate from URL hostname.
- `server_port`: Override server port for dialing.
- `token`: ==Required== Authentication token for the remote instance.
- `reverse`: Enable connector mode. Requires `url`. A connector dials out to `/ccm/v1/reverse` on the remote instance and cannot serve local requests directly. When `url` is set without `reverse`, the credential proxies requests through the remote instance normally and prefers an established reverse connection when one is available.
- `detour`: Outbound tag for connecting to the remote instance.
- `usages_path`: Optional usage tracking file.
- `poll_interval`: How often to poll the remote status endpoint. Default `30m`.

#### usages_path

Path to the file for storing aggregated API usage statistics.

Usage tracking is disabled if not specified.

When enabled, the service tracks and saves comprehensive statistics including:
- Request counts
- Token usage (input, output, cache read, cache creation)
- Calculated costs in USD based on Claude API pricing

Statistics are organized by model, context window (200k standard vs 1M premium), and optionally by user when authentication is enabled.

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
- `token`: Bearer token for authentication. Claude Code authenticates by setting the `ANTHROPIC_AUTH_TOKEN` environment variable to their token value.

!!! question "Since sing-box 1.14.0"

- `credential`: Credential tag to use for this user. ==Required== when `credentials` is set.
- `external_credential`: Tag of an external credential used only to rewrite response rate-limit headers with aggregated utilization from this user's other available credentials. It does not control request routing; request selection still comes from `credential` and `allow_external_usage`.
- `allow_external_usage`: Allow this user to use external credentials. `false` by default.

#### headers

Custom HTTP headers to send to the Claude API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the Claude API.

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
      "type": "ccm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "usages_path": "./claude-usages.json",
      "users": [
        {
          "name": "alice",
          "token": "ak-ccm-hello-world"
        },
        {
          "name": "bob",
          "token": "ak-ccm-hello-bob"
        }
      ]
    }
  ]
}
```

#### Client

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="ak-ccm-hello-world"

claude
```

### Example with Multiple Credentials

#### Server

```json
{
  "services": [
    {
      "type": "ccm",
      "listen": "0.0.0.0",
      "listen_port": 8080,
      "credentials": [
        {
          "tag": "a",
          "credential_path": "/home/user/.claude-a/.credentials.json",
          "usages_path": "/data/usages-a.json",
          "reserve_5h": 20,
          "reserve_weekly": 20
        },
        {
          "tag": "b",
          "credential_path": "/home/user/.claude-b/.credentials.json",
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
          "token": "ak-ccm-hello-world",
          "credential": "pool"
        },
        {
          "name": "bob",
          "token": "ak-ccm-hello-bob",
          "credential": "a"
        }
      ]
    }
  ]
}
```

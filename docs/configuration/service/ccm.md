---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# CCM

CCM (Claude Code Multiplexer) service is a multiplexing service that allows you to access your local Claude Code subscription remotely through custom tokens.

It handles OAuth authentication with Claude's API on your local machine while allowing remote Claude Code to authenticate using Auth Tokens via the `ANTHROPIC_AUTH_TOKEN` environment variable.

!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [credentials](#credentials)  
    :material-alert: [users](#users)

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
  "credential_path": "/path/to/.credentials.json",
  "usages_path": "/path/to/usages.json",
  "detour": "",
  "reserve_5h": 20,
  "reserve_weekly": 20
}
```

A single OAuth credential file. The `type` field can be omitted (defaults to `default`).

- `credential_path`: Path to the credentials file. Same defaults as top-level `credential_path`.
- `usages_path`: Optional usage tracking file for this credential.
- `detour`: Outbound tag for connecting to the Claude API with this credential.
- `reserve_5h`: Reserve threshold (1-99) for 5-hour window. Credential pauses at (100-N)% utilization.
- `reserve_weekly`: Reserve threshold (1-99) for weekly window. Credential pauses at (100-N)% utilization.

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
- Token usage (input, output, cache read, cache creation)
- Calculated costs in USD based on Claude API pricing

Statistics are organized by model, context window (200k standard vs 1M premium), and optionally by user when authentication is enabled.

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
- `token`: Bearer token for authentication. Claude Code authenticates by setting the `ANTHROPIC_AUTH_TOKEN` environment variable to their token value.
- `credential`: Credential tag to use for this user. ==Required== when `credentials` is set.

#### headers

Custom HTTP headers to send to the Claude API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the Claude API.

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

---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# CCM

CCM (Claude Code Multiplexer) service is a multiplexing service that allows you to access your local Claude Code subscription remotely through custom tokens.

It handles OAuth authentication with Claude's API on your local machine while allowing remote Claude Code to authenticate using Auth Tokens via the `ANTHROPIC_AUTH_TOKEN` environment variable.

### Structure

```json
{
  "type": "ccm",

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

Path to the Claude Code OAuth credentials file.

If not specified, defaults to:
- `$CLAUDE_CONFIG_DIR/.credentials.json` if `CLAUDE_CONFIG_DIR` environment variable is set
- `~/.claude/.credentials.json` otherwise

On macOS, credentials are read from the system keychain first, then fall back to the file if unavailable.

Refreshed tokens are automatically written back to the same location.

#### usages_path

Path to the file for storing aggregated API usage statistics.

Usage tracking is disabled if not specified.

When enabled, the service tracks and saves comprehensive statistics including:
- Request counts
- Token usage (input, output, cache read, cache creation)
- Calculated costs in USD based on Claude API pricing

Statistics are organized by model, context window (200k standard vs 1M premium), and optionally by user when authentication is enabled.

The statistics file is automatically saved every minute and upon service shutdown.

#### users

List of authorized users for token authentication.

If empty, no authentication is required.

Claude Code authenticates by setting the `ANTHROPIC_AUTH_TOKEN` environment variable to their token value.

#### headers

Custom HTTP headers to send to the Claude API.

These headers will override any existing headers with the same name.

#### detour

Outbound tag for connecting to the Claude API.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

### Example

```json
{
  "services": [
    {
      "type": "ccm",
      "listen": "127.0.0.1",
      "listen_port": 8080
    }
  ]
}
```

Connect to the CCM service:

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:8080"
export ANTHROPIC_AUTH_TOKEN="sk-ant-ccm-auth-token-not-required-in-this-context"

claude
```

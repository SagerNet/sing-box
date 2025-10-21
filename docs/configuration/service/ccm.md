---
icon: material/new-box
---

!!! question "Since sing-box 1.13.0"

# CCM

CCM (Claude Code Multiplexer) service is a proxy server that allows using API key authentication instead of OAuth for Claude Code API access.

It handles OAuth authentication with Claude's API and allows clients to authenticate using simple API keys via the `x-api-key` header.

### Structure

```json
{
  "type": "ccm",

  ... // Listen Fields

  "credential_path": "",
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

Claude Code OAuth credentials file path.

If not specified, uses `~/.claude/.credentials.json`.

On macOS, credentials are read from system keychain first, then fall back to file.

Refreshed tokens are written back to the same location.

#### users

List of users for API key authentication.

If empty, no authentication is performed.

Clients authenticate using `x-api-key` header with the token value.

#### headers

Custom HTTP headers to send to Claude API.

These headers override any existing headers with the same name.

#### detour

Outbound tag for connecting to Claude API.

#### tls

TLS configuration, see [TLS](/configuration/shared/tls/#inbound).

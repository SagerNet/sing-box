# Limiter

### Structure

```json
{
  "limiters": [
    {
      "tag": "limiter-a",
      "download": "10M",
      "upload": "1M",
      "auth_user": [
        "user-a",
        "user-b"
      ],
      "auth_user_independent": false,
      "inbound": [
        "in-a",
        "in-b"
      ],
      "inbound_independent": false
    }
  ]
}

```

### Fields

#### download upload

==Required==

Format: `[Integer][Unit]` e.g. `100M, 100m, 1G, 1g`.

Supported units (case insensitive): `B, K, M, G, T, P, E`.

#### tag

The tag of the limiter, used in route rule.

#### auth_user

Apply limiter for a group of usernames, see each inbound for details.

#### auth_user_independent

Make each auth_user's limiter independent. If disabled, the same limiter will be shared.

#### inbound

Apply limiter for a group of inbounds.

#### inbound_independent

Make each inbound's limiter independent. If disabled, the same limiter will be shared.

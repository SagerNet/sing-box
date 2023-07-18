# Limiter

### Structure

```json
{
  "limiters": [
    {
      "tag": "limiter-a",
      "download": "1M",
      "upload": "10M",
      "auth_user": [
        "user-a",
        "user-b"
      ],
      "inbound": [
        "in-a",
        "in-b"
      ]
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

Global limiter for a group of usernames, see each inbound for details.

#### inbound

Global limiter for a group of inbounds.

!!! info ""

    All the auth_users, inbounds and route rule with limiter tag share the same limiter. To take effect independently, configure limiters seperately.
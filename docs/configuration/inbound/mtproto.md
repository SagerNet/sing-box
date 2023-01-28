### Structure

```json
{
  "type": "mtproto",
  "tag": "mtproto-in",

  ... // Listen Fields

  "users": [
    {
      "name": "sekai",
      "secret": "ee134132e79f44020784bddce2e734b5e2676f6f676c652e636f6d"
    }
  ]
}
```

!!! warning ""

    MTProto is not included by default, see [Installation](/#installation).

### Listen Fields

See [Listen Fields](/configuration/shared/listen) for details.

### Fields

#### users

==Required==

MTProto users, where secret is a MTProto V3 secret.

!!! note ""

    MTProto multi-user inbound might be poor on performance limited to its authentication algorithm.

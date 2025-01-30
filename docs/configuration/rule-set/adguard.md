!!! question "Since sing-box 1.10.0"

sing-box supports some rule-set formats from other projects which cannot be fully translated to sing-box,
currently only AdGuard DNS Filter.

These formats are not directly supported as source formats,
instead you need to convert them to binary rule-set.

## Convert

Use `sing-box rule-set convert --type adguard [--output <file-name>.srs] <file-name>.txt` to convert to binary rule-set.

## Performance

AdGuard keeps all rules in memory and matches them sequentially,
while sing-box chooses high performance and smaller memory usage.
As a trade-off, you cannot know which rule item is matched.

## Compatibility

Almost all rules in [AdGuardSDNSFilter](https://github.com/AdguardTeam/AdGuardSDNSFilter)
and rules in rule-sets listed in [adguard-filter-list](https://github.com/ppfeufer/adguard-filter-list)
are supported.

## Supported formats

### AdGuard Filter

#### Basic rule syntax

| Syntax | Supported        |
|--------|------------------|
| `@@`   | :material-check: | 
| `\|\|` | :material-check: | 
| `\|`   | :material-check: |
| `^`    | :material-check: |
| `*`    | :material-check: |

#### Host syntax

| Syntax      | Example                  | Supported                |
|-------------|--------------------------|--------------------------|
| Scheme      | `https://`               | :material-alert: Ignored |
| Domain Host | `example.org`            | :material-check:         |
| IP Host     | `1.1.1.1`, `10.0.0.`     | :material-close:         |
| Regexp      | `/regexp/`               | :material-check:         |
| Port        | `example.org:80`         | :material-close:         |
| Path        | `example.org/path/ad.js` | :material-close:         |

#### Modifier syntax

| Modifier              | Supported                |
|-----------------------|--------------------------|
| `$important`          | :material-check:         |
| `$dnsrewrite=0.0.0.0` | :material-alert: Ignored |
| Any other modifiers   | :material-close:         |

### Hosts

Only items with `0.0.0.0` IP addresses will be accepted.

### Simple

When all rule lines are valid domains, they are treated as simple line-by-line domain rules which,
like hosts, only match the exact same domain.
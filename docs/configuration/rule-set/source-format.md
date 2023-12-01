---
icon: material/new-box
---

# Source Format

!!! question "Since sing-box 1.8.0"

### Structure

```json
{
  "version": 1,
  "rules": []
}
```

### Compile

Use `sing-box rule-set compile [--output <file-name>.srs] <file-name>.json` to compile source to binary rule-set.

### Fields

#### version

==Required==

Version of Rule Set, must be `1`.

#### rules

==Required==

List of [Headless Rule](./headless-rule.md).

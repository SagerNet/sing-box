---
icon: material/new-box
---

# Source Format

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: version `2`

!!! question "Since sing-box 1.8.0"

### Structure

```json
{
  "version": 2,
  "rules": []
}
```

### Compile

Use `sing-box rule-set compile [--output <file-name>.srs] <file-name>.json` to compile source to binary rule-set.

### Fields

#### version

==Required==

Version of rule-set, one of `1` or `2`.

* 1: Initial rule-set version, since sing-box 1.8.0.
* 2: Optimized memory usages of `domain_suffix` rules.

The new rule-set version `2` does not make any changes to the format, only affecting `binary` rule-sets compiled by command `rule-set compile`

Since 1.10.0, the optimization is always applied to `source` rule-sets even if version is set to `1`.

It is recommended to upgrade to `2` after sing-box 1.10.0 becomes a stable version.

#### rules

==Required==

List of [Headless Rule](./headless-rule.md/).

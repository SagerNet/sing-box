---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.13.0"

    :material-plus: version `4`

!!! quote "Changes in sing-box 1.11.0"

    :material-plus: version `3`

!!! quote "Changes in sing-box 1.10.0"

    :material-plus: version `2`

!!! question "Since sing-box 1.8.0"

### Structure

```json
{
  "version": 3,
  "rules": []
}
```

### Compile

Use `sing-box rule-set compile [--output <file-name>.srs] <file-name>.json` to compile source to binary rule-set.

### Fields

#### version

==Required==

Version of rule-set.

* 1: sing-box 1.8.0: Initial rule-set version.
* 2: sing-box 1.10.0: Optimized memory usages of `domain_suffix` rules in binary rule-sets.
* 3: sing-box 1.11.0: Added `network_type`, `network_is_expensive` and `network_is_constrainted` rule items.
* 4: sing-box 1.13.0: Added `network_interface_address` and `default_interface_address` rule items.

#### rules

==Required==

List of [Headless Rule](../headless-rule/).

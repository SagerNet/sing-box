---
icon: material/new-box
---

!!! question "Since sing-box 1.14.0"

# JSON Schema

sing-box provides a JSON Schema Draft 2020-12 for configuration files.
Compatible editors can use it for completion and validation.

### Structure

```json
{
  "$schema": "https://sing-box.sagernet.org/schema.json"
}
```

### Fields

#### $schema

The schema URI used by compatible editors.
This field does not affect sing-box runtime behavior.

The schema published with this documentation is available at
[sing-box.sagernet.org/schema.json](https://sing-box.sagernet.org/schema.json).

### Generate

Use the following command to generate a schema matching the installed binary:

```bash
sing-box schema -o schema.json
```

Without `--output`, the schema is written to standard output.
The generated schema reflects the features included in the current build.

You can then reference the local schema from a configuration file:

```json
{
  "$schema": "./schema.json"
}
```

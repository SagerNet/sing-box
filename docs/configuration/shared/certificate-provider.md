---
icon: material/new-box
---

!!! quote "Changes in sing-box 1.14.0"

    :material-alert: [certificate_provider](#certificate_provider)

### Structure

```json
{
  "certificate_provider": "my-cert"
}
```

Or

```json
{
  "certificate_provider": {
    "type": "acme",

    ... // Provider Fields
  }
}
```

### Fields

#### certificate_provider

A string or an object.

When string, the tag of a shared [Certificate Provider](/configuration/certificate-provider/).

When object, an inline certificate provider. See [Certificate Provider](/configuration/certificate-provider/) for available types and fields.

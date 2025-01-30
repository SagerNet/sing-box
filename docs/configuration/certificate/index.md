---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Certificate

### Structure

```json
{
  "store": "",
  "certificate": [],
  "certificate_path": [],
  "certificate_directory_path": []
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Fields

#### store

The default X509 trusted CA certificate list.

| Type               | Description                                                                                                   |
|--------------------|---------------------------------------------------------------------------------------------------------------|
| `system` (default) | System trusted CA certificates                                                                                |
| `mozilla`          | [Mozilla Included List](https://wiki.mozilla.org/CA/Included_Certificates) with China CA certificates removed |
| `none`             | Empty list                                                                                                    |

#### certificate

The certificate line array to trust, in PEM format.

#### certificate_path

!!! note ""

    Will be automatically reloaded if file modified.

The paths to certificates to trust, in PEM format.

#### certificate_directory_path

!!! note ""

    Will be automatically reloaded if file modified.

The directory path to search for certificates to trust,in PEM format.

---
icon: material/new-box
---

!!! question "Since sing-box 1.12.0"

# Predefined

### Structure

```json
{
  "dns": {
    "servers": [
      {
        "type": "predefined",
        "tag": "",
        "responses": []
      }
    ]
  }
}
```

### Fields

#### responses

==Required==

List of [Response](#response-structure).

### Response Structure

```json
{
  "query": [],
  "query_type": [],
  "rcode": "",
  "answer": [],
  "ns": [],
  "extra": []
}
```

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### Response Fields

#### query

List of domain name to match.

#### query_type

List of query type to match.

#### rcode

The response code.

| Value      | Value in the legacy rcode server | Description     |
|------------|----------------------------------|-----------------|
| `NOERROR`  | `success`                        | Ok              |
| `FORMERR`  | `format_error`                   | Bad request     |
| `SERVFAIL` | `server_failure`                 | Server failure  |
| `NXDOMAIN` | `name_error`                     | Not found       |
| `NOTIMP`   | `not_implemented`                | Not implemented |
| `REFUSED`  | `refused`                        | Refused         |

`NOERROR` will be used by default.

#### answer

List of text DNS record to respond as answers.

Examples:

| Record Type | Example                       |
|-------------|-------------------------------|
| `A`         | `localhost. IN A 127.0.0.1`   |
| `AAAA`      | `localhost. IN AAAA ::1`      |
| `TXT`       | `localhost. IN TXT \"Hello\"` |

#### ns

List of text DNS record to respond as name servers.

#### extra

List of text DNS record to respond as extra records.

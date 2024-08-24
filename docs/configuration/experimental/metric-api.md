
### Structure

```json
{
  "listen": ":8080",
  "path": "/metrics"
}
```

### Fields

#### listen

Prometheus metrics API listening address, disabled if empty.

#### path

Prometheus scrape path, `/metrics` will be used if empty.

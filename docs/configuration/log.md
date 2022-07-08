# Log

### Structure

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "timestamp": true
  }
}

```

### Fields

#### disabled

Disable logging, no output after start.

#### level

Log level. One of: `trace` `debug` `info` `warn` `error` `fatal` `panic`.

#### output

Output file path. Will not write log to console after enable.

#### timestamp

Add time to each line.
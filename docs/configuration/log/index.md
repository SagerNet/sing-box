# Log

### Structure

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "timestamp": true,
    "access": {
      "enabled": true,
      "path": "access"
    }
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

#### access

Dedicated access log output.

### Access Fields

#### enabled

Enable access log output.

#### path

Access log directory. Files are written in JSON Lines format, rotated hourly as `access-YYYY-MM-DD-HH.log`, and kept for 7 days.

# Specification

## Profile

Profile defines a sing-box configuration with metadata in a GUI client.

## Profile Types

### Local

Create a empty configuration or import from a local file.

### iCloud (on Apple platforms)

Create a new configuration or use an existing configuration on iCloud. 

### Remote

Use a remote URL as the configuration source, with HTTP basic authentication and automatic update support.

#### URL specification

```
sing-box://import-remote-profile?url=urlEncodedURL#urlEncodedName
```
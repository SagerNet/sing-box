# IPScanner

IPScanner is a Go package designed for scanning and analyzing IP addresses. It utilizes various dialers and an internal engine to perform scans efficiently.

## Features
- IPv4 and IPv6 support.
- Customizable timeout and dialer options.
- Extendable with various ping methods (HTTP, QUIC, TCP, TLS).
- Adjustable IP Queue size for scan optimization.

## Getting Started
To use IPScanner, simply import the package and initialize a new scanner with your desired options.

```go
import "github.com/bepass-org/warp-plus/ipscanner"

func main() {
    scanner := ipscanner.NewScanner(
        // Configure your options here
    )
    scanner.Run()
}
```

## Options
You can customize your scanner with several options:
- `WithUseIPv4` and `WithUseIPv6` to specify IP versions.
- `WithDialer` and `WithTLSDialer` to define custom dialing functions.
- `WithTimeout` to set the scan timeout.
- `WithIPQueueSize` to set the IP Queue size.
- `WithPingMethod` to set the ping method, it can be HTTP, QUIC, TCP, TLS at the same time.
- Various other options for detailed scan control.

## Contributing
Contributions to IPScanner are welcome. Please ensure to follow the project's coding standards and submit detailed pull requests.

## License
IPScanner is licensed under the MIT license. See [LICENSE](LICENSE) for more information.

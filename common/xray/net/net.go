package net

import "time"

// defines the maximum time an idle TCP session can survive in the tunnel, so
// it should be consistent across HTTP versions and with other transports.
const ConnIdleTimeout = 300 * time.Second

// consistent with quic-go
const QuicgoH3KeepAlivePeriod = 10 * time.Second

// consistent with chrome
const ChromeH2KeepAlivePeriod = 45 * time.Second

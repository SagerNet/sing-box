package constant

const (
	V2RayTransportTypeHTTP        = "http"
	V2RayTransportTypeWebsocket   = "ws"
	V2RayTransportTypeQUIC        = "quic"
	V2RayTransportTypeGRPC        = "grpc"
	V2RayTransportTypeHTTPUpgrade = "httpupgrade"
	V2RayTransportTypeXHTTP       = "xhttp"
)

// XHTTP (SplitHTTP) mode constants
const (
	XHTTPModeAuto     = "auto"
	XHTTPModePacketUp = "packet-up"
	XHTTPModeStreamUp = "stream-up"
	XHTTPModeStreamOne = "stream-one"
)

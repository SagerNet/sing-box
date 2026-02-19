package nftables

import (
	"net/netip"
	"time"

	"github.com/sagernet/sing/common/logger"
)

// Manager provides cross-platform nftables set management
type Manager interface {
	// Start initializes the manager
	Start() error

	// Close cleans up resources
	Close() error

	// AddAddresses adds an IP addresses to a named set
	AddAddress(setName string, address netip.Addr, ttl time.Duration, reason string) error

	Flush() error
}

// Options for creating the manager
type Options struct {
	Logger logger.ContextLogger
}

// NewManager creates a platform-appropriate nftables manager
func NewManager(options Options) (Manager, error) {
	return newManager(options)
}

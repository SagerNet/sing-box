package v2raykcp

import (
	"crypto/cipher"

	"github.com/sagernet/sing-box/option"
)

// Config stores the configurations for KCP transport
type Config struct {
	MTU              uint32
	TTI              uint32
	UplinkCapacity   uint32
	DownlinkCapacity uint32
	Congestion       bool
	ReadBufferSize   uint32
	WriteBufferSize  uint32
	HeaderType       string
	Seed             string
}

// NewConfig creates a new Config from options
func NewConfig(options option.V2RayKCPOptions) *Config {
	return &Config{
		MTU:              options.GetMTU(),
		TTI:              options.GetTTI(),
		UplinkCapacity:   options.GetUplinkCapacity(),
		DownlinkCapacity: options.GetDownlinkCapacity(),
		Congestion:       options.Congestion,
		ReadBufferSize:   options.GetReadBufferSize(),
		WriteBufferSize:  options.GetWriteBufferSize(),
		HeaderType:       options.GetHeaderType(),
		Seed:             options.Seed,
	}
}

// GetMTUValue returns the value of MTU settings.
func (c *Config) GetMTUValue() uint32 {
	if c == nil || c.MTU == 0 {
		return 1350
	}
	return c.MTU
}

// GetTTIValue returns the value of TTI settings.
func (c *Config) GetTTIValue() uint32 {
	if c == nil || c.TTI == 0 {
		return 50
	}
	return c.TTI
}

// GetUplinkCapacityValue returns the value of UplinkCapacity settings.
func (c *Config) GetUplinkCapacityValue() uint32 {
	if c == nil || c.UplinkCapacity == 0 {
		return 12
	}
	return c.UplinkCapacity
}

// GetDownlinkCapacityValue returns the value of DownlinkCapacity settings.
func (c *Config) GetDownlinkCapacityValue() uint32 {
	if c == nil || c.DownlinkCapacity == 0 {
		return 100
	}
	return c.DownlinkCapacity
}

// GetWriteBufferSize returns the size of WriterBuffer in bytes.
func (c *Config) GetWriteBufferSize() uint32 {
	if c == nil || c.WriteBufferSize == 0 {
		return 2 * 1024 * 1024
	}
	return c.WriteBufferSize * 1024 * 1024
}

// GetReadBufferSize returns the size of ReadBuffer in bytes.
func (c *Config) GetReadBufferSize() uint32 {
	if c == nil || c.ReadBufferSize == 0 {
		return 2 * 1024 * 1024
	}
	return c.ReadBufferSize * 1024 * 1024
}

// GetSecurity returns the security settings.
func (c *Config) GetSecurity() (cipher.AEAD, error) {
	if c.Seed != "" {
		return NewAEADAESGCMBasedOnSeed(c.Seed), nil
	}
	return NewSimpleAuthenticator(), nil
}

// GetHeaderType returns the header type
func (c *Config) GetHeaderType() string {
	if c.HeaderType == "" {
		return "none"
	}
	return c.HeaderType
}

// GetPacketHeader builds a new PacketHeader for this config.
func (c *Config) GetPacketHeader() PacketHeader {
	return NewPacketHeader(c.GetHeaderType())
}

func (c *Config) GetSendingInFlightSize() uint32 {
	size := c.GetUplinkCapacityValue() * 1024 * 1024 / c.GetMTUValue() / (1000 / c.GetTTIValue())
	if size < 8 {
		size = 8
	}
	return size
}

func (c *Config) GetSendingBufferSize() uint32 {
	return c.GetWriteBufferSize() / c.GetMTUValue()
}

func (c *Config) GetReceivingInFlightSize() uint32 {
	size := c.GetDownlinkCapacityValue() * 1024 * 1024 / c.GetMTUValue() / (1000 / c.GetTTIValue())
	if size < 8 {
		size = 8
	}
	return size
}

func (c *Config) GetReceivingBufferSize() uint32 {
	return c.GetReadBufferSize() / c.GetMTUValue()
}

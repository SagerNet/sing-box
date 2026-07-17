//go:build !with_gvisor

package openvpn

import E "github.com/sagernet/sing/common/exceptions"

func newStackDevice(options DeviceOptions) (Device, error) {
	return nil, E.New("OpenVPN system:false requires the with_gvisor build tag")
}

func newSystemStackDevice(options DeviceOptions) (Device, error) {
	return nil, E.New("OpenVPN system stack requires the with_gvisor build tag")
}

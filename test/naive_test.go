package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/network"
)

func TestNaiveInbound(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeNaive,
				NaiveOptions: option.NaiveInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []auth.User{
						{
							Username: "sekai",
							Password: "password",
						},
					},
					Network: network.NetworkTCP,
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		},
	})
	startDockerContainer(t, DockerOptions{
		Image: ImageNaive,
		Ports: []uint16{serverPort, clientPort},
		Bind: map[string]string{
			"naive.json": "/etc/naiveproxy/config.json",
			caPem:        "/etc/naiveproxy/ca.pem",
		},
		Env: []string{
			"SSL_CERT_FILE=/etc/naiveproxy/ca.pem",
		},
	})
	testTCP(t, clientPort, testPort)
}

func TestNaiveHTTP3Inbound(t *testing.T) {
	if !C.QUIC_AVAILABLE {
		t.Skip("QUIC not included")
	}
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Log: &option.LogOptions{
			Level: "error",
		},
		Inbounds: []option.Inbound{
			{
				Type: C.TypeNaive,
				NaiveOptions: option.NaiveInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     option.ListenAddress(netip.IPv4Unspecified()),
						ListenPort: serverPort,
					},
					Users: []auth.User{
						{
							Username: "sekai",
							Password: "password",
						},
					},
					Network: network.NetworkUDP,
					TLS: &option.InboundTLSOptions{
						Enabled:         true,
						ServerName:      "example.org",
						CertificatePath: certPem,
						KeyPath:         keyPem,
					},
				},
			},
		},
	})
	startDockerContainer(t, DockerOptions{
		Image: ImageNaive,
		Ports: []uint16{serverPort, clientPort},
		Bind: map[string]string{
			"naive-quic.json": "/etc/naiveproxy/config.json",
			caPem:             "/etc/naiveproxy/ca.pem",
		},
		Env: []string{
			"SSL_CERT_FILE=/etc/naiveproxy/ca.pem",
		},
	})
	testTCP(t, clientPort, testPort)
}

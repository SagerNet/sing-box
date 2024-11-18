package main

import (
	"net/netip"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/network"
)

func TestNaiveInboundWithNginx(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeNaive,
				Options: &option.NaiveInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: otherPort,
					},
					Users: []auth.User{
						{
							Username: "sekai",
							Password: "password",
						},
					},
					Network: network.NetworkTCP,
				},
			},
		},
	})
	startDockerContainer(t, DockerOptions{
		Image: ImageNginx,
		Ports: []uint16{serverPort, otherPort},
		Bind: map[string]string{
			"nginx.conf":       "/etc/nginx/nginx.conf",
			"naive-nginx.conf": "/etc/nginx/conf.d/naive.conf",
			certPem:            "/etc/nginx/cert.pem",
			keyPem:             "/etc/nginx/key.pem",
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

func TestNaiveInbound(t *testing.T) {
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeNaive,
				Options: &option.NaiveInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []auth.User{
						{
							Username: "sekai",
							Password: "password",
						},
					},
					Network: network.NetworkTCP,
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
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
	caPem, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeNaive,
				Options: &option.NaiveInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []auth.User{
						{
							Username: "sekai",
							Password: "password",
						},
					},
					Network: network.NetworkUDP,
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
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

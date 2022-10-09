package outbound

import (
	"context"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/crypto/ssh"
)

var _ adapter.Outbound = (*SSH)(nil)

type SSH struct {
	myOutboundAdapter
	ctx               context.Context
	dialer            N.Dialer
	serverAddr        M.Socksaddr
	user              string
	hostKeyAlgorithms []string
	clientVersion     string
	authMethod        []ssh.AuthMethod
	clientAccess      sync.Mutex
	clientConn        net.Conn
	client            *ssh.Client
}

func NewSSH(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.SSHOutboundOptions) (*SSH, error) {
	outbound := &SSH{
		myOutboundAdapter: myOutboundAdapter{
			protocol: C.TypeSSH,
			network:  []string{N.NetworkTCP},
			router:   router,
			logger:   logger,
			tag:      tag,
		},
		ctx:               ctx,
		dialer:            dialer.New(router, options.DialerOptions),
		serverAddr:        options.ServerOptions.Build(),
		user:              options.User,
		hostKeyAlgorithms: options.HostKeyAlgorithms,
		clientVersion:     options.ClientVersion,
	}
	if outbound.serverAddr.Port == 0 {
		outbound.serverAddr.Port = 22
	}
	if outbound.user == "" {
		outbound.user = "root"
	}
	if outbound.clientVersion == "" {
		outbound.clientVersion = randomVersion()
	}
	if options.Password != "" {
		outbound.authMethod = append(outbound.authMethod, ssh.Password(options.Password))
	}
	if options.PrivateKey != "" || options.PrivateKeyPath != "" {
		var privateKey []byte
		if options.PrivateKey != "" {
			privateKey = []byte(options.PrivateKey)
		} else {
			var err error
			privateKey, err = os.ReadFile(os.ExpandEnv(options.PrivateKeyPath))
			if err != nil {
				return nil, E.Cause(err, "read private key")
			}
		}
		var signer ssh.Signer
		var err error
		if options.PrivateKeyPassphrase == "" {
			signer, err = ssh.ParsePrivateKey(privateKey)
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKey, []byte(options.PrivateKeyPassphrase))
		}
		if err != nil {
			return nil, E.Cause(err, "parse private key")
		}
		outbound.authMethod = append(outbound.authMethod, ssh.PublicKeys(signer))
	}
	return outbound, nil
}

func randomVersion() string {
	version := "SSH-2.0-OpenSSH_"
	if rand.Intn(2) == 0 {
		version += "7." + strconv.Itoa(rand.Intn(10))
	} else {
		version += "8." + strconv.Itoa(rand.Intn(9))
	}
	return version
}

func (s *SSH) connect() (*ssh.Client, error) {
	if s.client != nil {
		return s.client, nil
	}

	s.clientAccess.Lock()
	defer s.clientAccess.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	conn, err := s.dialer.DialContext(s.ctx, N.NetworkTCP, s.serverAddr)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:              s.user,
		Auth:              s.authMethod,
		ClientVersion:     s.clientVersion,
		HostKeyAlgorithms: s.hostKeyAlgorithms,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, s.serverAddr.Addr.String(), config)
	if err != nil {
		conn.Close()
		return nil, E.Cause(err, "connect to ssh server")
	}

	client := ssh.NewClient(clientConn, chans, reqs)

	s.clientConn = conn
	s.client = client

	go func() {
		client.Wait()
		conn.Close()
		s.clientAccess.Lock()
		s.client = nil
		s.clientConn = nil
		s.clientAccess.Unlock()
	}()

	return client, nil
}

func (s *SSH) Close() error {
	return common.Close(s.clientConn)
}

func (s *SSH) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	client, err := s.connect()
	if err != nil {
		return nil, err
	}
	return client.Dial(network, destination.String())
}

func (s *SSH) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (s *SSH) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, s, conn, metadata)
}

func (s *SSH) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}

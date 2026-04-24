//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

type standardTestDialer struct{}

func (standardTestDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, destination.String())
}

func (standardTestDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

func TestClientStandardSessionPollsDevList(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	entry := standardTestDeviceEntry("1-1")
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveStandardDevLists(listener, [][]DeviceEntry{
			nil,
			{entry},
		})
	}()

	target := clientTarget{fixedBusID: "1-1"}
	worker := &clientAssignedWorker{target: target, updates: make(chan string, 1)}
	client := &ClientService{
		ctx:             ctx,
		logger:          log.NewNOPFactory().NewLogger("usbip"),
		dialer:          standardTestDialer{},
		serverAddr:      standardTestSocksaddr(listener.Addr()),
		matches:         []option.USBIPDeviceMatch{{BusID: "1-1"}},
		targets:         []clientTarget{target},
		assigned:        []string{""},
		assignedWorkers: []*clientAssignedWorker{worker},
		allWorkers:      make(map[string]*clientBusIDWorker),
		allDesired:      make(map[string]struct{}),
		activeBusIDs:    make(map[string]struct{}),
	}

	sessionErr := make(chan error, 1)
	go func() {
		sessionErr <- client.runStandardSessionWithInterval(10 * time.Millisecond)
	}()

	select {
	case update := <-worker.updates:
		require.Equal(t, "1-1", update)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for standard devlist refresh")
	}
	cancel()

	select {
	case err := <-sessionErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for standard session shutdown")
	}
	require.NoError(t, <-serverErr)
}

func serveStandardDevLists(listener net.Listener, responses [][]DeviceEntry) error {
	for _, entries := range responses {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		if err := handleStandardDevListConn(conn, entries); err != nil {
			return err
		}
	}
	return nil
}

func handleStandardDevListConn(conn net.Conn, entries []DeviceEntry) error {
	defer conn.Close()
	header, err := ReadOpHeader(conn)
	if err != nil {
		return err
	}
	if header.Version != ProtocolVersion || header.Code != OpReqDevList || header.Status != OpStatusOK {
		return fmt.Errorf("unexpected devlist request: version=0x%s code=0x%s status=%d", hex16(header.Version), hex16(header.Code), header.Status)
	}
	return WriteOpRepDevList(conn, entries)
}

func standardTestDeviceEntry(busid string) DeviceEntry {
	var info DeviceInfoTruncated
	copy(info.BusID[:], busid)
	info.BusNum = 1
	info.DevNum = 1
	info.Speed = SpeedHigh
	info.IDVendor = 0x1d6b
	info.IDProduct = 0x0002
	info.BConfigurationValue = 1
	info.BNumConfigurations = 1
	info.BNumInterfaces = 1
	return DeviceEntry{
		Info: info,
		Interfaces: []DeviceInterface{{
			BInterfaceClass: 0xff,
		}},
	}
}

func standardTestSocksaddr(address net.Addr) M.Socksaddr {
	tcpAddr := address.(*net.TCPAddr)
	return M.ParseSocksaddrHostPort(tcpAddr.IP.String(), uint16(tcpAddr.Port))
}

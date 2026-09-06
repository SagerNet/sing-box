//go:build linux

package process

import (
	"context"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"

	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

func socketInode(t *testing.T, conn syscall.Conn) uint32 {
	rawConn, err := conn.SyscallConn()
	require.NoError(t, err)
	var inode uint32
	err = rawConn.Control(func(fd uintptr) {
		var stat syscall.Stat_t
		require.NoError(t, syscall.Fstat(int(fd), &stat))
		inode = uint32(stat.Ino)
	})
	require.NoError(t, err)
	return inode
}

func TestDumpSocketDiagTCP(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	first, err := (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 2)}}).Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer first.Close()
	second, err := (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 3), Port: int(M.AddrPortFromNet(first.LocalAddr()).Port())}}).Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer second.Close()
	destination := M.AddrPortFromNet(listener.Addr())
	for _, conn := range []net.Conn{second, first} {
		inode, uid, err := dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_TCP, M.AddrPortFromNet(conn.LocalAddr()), destination)
		require.NoError(t, err)
		require.Equal(t, socketInode(t, conn.(syscall.Conn)), inode)
		require.Equal(t, uint32(os.Getuid()), uid)
		inode, _, err = dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_TCP, M.AddrPortFromNet(conn.LocalAddr()), netip.AddrPort{})
		require.NoError(t, err)
		require.Equal(t, socketInode(t, conn.(syscall.Conn)), inode)
	}
	_, _, err = dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_TCP, netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 4}), M.AddrPortFromNet(first.LocalAddr()).Port()), destination)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDumpSocketDiagUDP(t *testing.T) {
	t.Parallel()
	first, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2)})
	require.NoError(t, err)
	defer first.Close()
	port := M.AddrPortFromNet(first.LocalAddr()).Port()
	second, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 3), Port: int(port)})
	require.NoError(t, err)
	defer second.Close()
	for _, conn := range []*net.UDPConn{second, first} {
		inode, _, err := dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_UDP, M.AddrPortFromNet(conn.LocalAddr()), netip.AddrPort{})
		require.NoError(t, err)
		require.Equal(t, socketInode(t, conn), inode)
	}
	_, _, err = dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_UDP, netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 4}), port), netip.AddrPort{})
	require.ErrorIs(t, err, ErrNotFound)

	wildcard, err := net.ListenUDP("udp4", &net.UDPAddr{})
	require.NoError(t, err)
	defer wildcard.Close()
	inode, _, err := dumpSocketDiag(syscall.AF_INET, syscall.IPPROTO_UDP, netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 4}), M.AddrPortFromNet(wildcard.LocalAddr()).Port()), netip.AddrPort{})
	require.NoError(t, err)
	require.Equal(t, socketInode(t, wildcard), inode)
}

func TestLinuxSearcherFindProcessInfo(t *testing.T) {
	t.Parallel()
	searcher, err := NewSearcher(Config{Logger: log.NewNOPFactory().NewLogger("test")})
	require.NoError(t, err)
	defer searcher.Close()
	executable, err := os.Executable()
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	tcpConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer tcpConn.Close()
	info, err := searcher.FindProcessInfo(context.Background(), N.NetworkTCP, M.AddrPortFromNet(tcpConn.LocalAddr()), M.AddrPortFromNet(listener.Addr()))
	require.NoError(t, err)
	require.Equal(t, []string{executable}, info.ProcessPaths)
	require.Equal(t, int32(os.Getuid()), info.UserId)

	udpConn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	require.NoError(t, err)
	defer udpConn.Close()
	info, err = searcher.FindProcessInfo(context.Background(), N.NetworkUDP, netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), M.AddrPortFromNet(udpConn.LocalAddr()).Port()), netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 53))
	require.NoError(t, err)
	require.Equal(t, []string{executable}, info.ProcessPaths)
}

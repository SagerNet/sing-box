package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"errors"
	"io"
	"net"
	_ "net/http/pprof"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/control"
	F "github.com/sagernet/sing/common/format"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kanged from clash

const (
	ImageShadowsocksRustServer = "ghcr.io/shadowsocks/ssserver-rust:latest"
	ImageShadowsocksRustClient = "ghcr.io/shadowsocks/sslocal-rust:latest"
	ImageV2RayCore             = "v2fly/v2fly-core:latest"
	ImageTrojan                = "trojangfw/trojan:latest"
	ImageNaive                 = "pocat/naiveproxy:client"
	ImageBoringTun             = "ghcr.io/ntkme/boringtun:edge"
	ImageHysteria              = "tobyxdd/hysteria:v1.3.5"
	ImageHysteria2             = "tobyxdd/hysteria:v2"
	ImageNginx                 = "nginx:stable"
	ImageShadowTLS             = "ghcr.io/ihciah/shadow-tls:latest"
	ImageXRayCore              = "teddysun/xray:latest"
	ImageShadowsocksLegacy     = "mritd/shadowsocks:latest"
	ImageTUICServer            = "kilvn/tuic-server:latest"
	ImageTUICClient            = "kilvn/tuic-client:latest"
)

var allImages = []string{
	ImageShadowsocksRustServer,
	ImageShadowsocksRustClient,
	ImageV2RayCore,
	ImageTrojan,
	ImageNaive,
	ImageBoringTun,
	ImageHysteria,
	ImageHysteria2,
	ImageNginx,
	ImageShadowTLS,
	ImageXRayCore,
	ImageShadowsocksLegacy,
	ImageTUICServer,
	ImageTUICClient,
}

var localIP = netip.MustParseAddr("127.0.0.1")

func init() {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer dockerClient.Close()

	list, err := dockerClient.ImageList(context.Background(), image.ListOptions{All: true})
	if err != nil {
		log.Warn(err)
		return
	}

	imageExist := func(image string) bool {
		for _, item := range list {
			for _, tag := range item.RepoTags {
				if image == tag {
					return true
				}
			}
		}
		return false
	}

	for _, i := range allImages {
		if imageExist(i) {
			continue
		}

		log.Info("pulling image: ", i)
		imageStream, err := dockerClient.ImagePull(context.Background(), i, image.PullOptions{})
		if err != nil {
			panic(err)
		}

		io.Copy(io.Discard, imageStream)
	}
}

func newPingPongPair() (chan []byte, chan []byte, func(t *testing.T) error) {
	pingCh := make(chan []byte)
	pongCh := make(chan []byte)
	test := func(t *testing.T) error {
		defer close(pingCh)
		defer close(pongCh)
		pingOpen := false
		pongOpen := false
		var recv []byte

		for {
			if pingOpen && pongOpen {
				break
			}

			select {
			case recv, pingOpen = <-pingCh:
				assert.True(t, pingOpen)
				assert.Equal(t, []byte("ping"), recv)
			case recv, pongOpen = <-pongCh:
				assert.True(t, pongOpen)
				assert.Equal(t, []byte("pong"), recv)
			case <-time.After(10 * time.Second):
				return errors.New("timeout")
			}
		}
		return nil
	}

	return pingCh, pongCh, test
}

func newLargeDataPair() (chan hashPair, chan hashPair, func(t *testing.T) error) {
	pingCh := make(chan hashPair)
	pongCh := make(chan hashPair)
	test := func(t *testing.T) error {
		defer close(pingCh)
		defer close(pongCh)
		pingOpen := false
		pongOpen := false
		var serverPair hashPair
		var clientPair hashPair

		for {
			if pingOpen && pongOpen {
				break
			}

			select {
			case serverPair, pingOpen = <-pingCh:
				assert.True(t, pingOpen)
			case clientPair, pongOpen = <-pongCh:
				assert.True(t, pongOpen)
			case <-time.After(10 * time.Second):
				return errors.New("timeout")
			}
		}

		assert.Equal(t, serverPair.recvHash, clientPair.sendHash)
		assert.Equal(t, serverPair.sendHash, clientPair.recvHash)

		return nil
	}

	return pingCh, pongCh, test
}

func testPingPongWithConn(t *testing.T, port uint16, cc func() (net.Conn, error)) error {
	l, err := listen("tcp", ":"+F.ToString(port))
	if err != nil {
		return err
	}
	defer l.Close()

	c, err := cc()
	if err != nil {
		return err
	}
	defer c.Close()

	pingCh, pongCh, test := newPingPongPair()
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}

		buf := make([]byte, 4)
		if _, err := io.ReadFull(c, buf); err != nil {
			return
		}

		pingCh <- buf
		if _, err := c.Write([]byte("pong")); err != nil {
			return
		}
	}()

	go func() {
		if _, err := c.Write([]byte("ping")); err != nil {
			return
		}

		buf := make([]byte, 4)
		if _, err := io.ReadFull(c, buf); err != nil {
			return
		}

		pongCh <- buf
	}()

	return test(t)
}

func testPingPongWithPacketConn(t *testing.T, port uint16, pcc func() (net.PacketConn, error)) error {
	l, err := listenPacket("udp", ":"+F.ToString(port))
	if err != nil {
		return err
	}
	defer l.Close()

	rAddr := &net.UDPAddr{IP: localIP.AsSlice(), Port: int(port)}

	pingCh, pongCh, test := newPingPongPair()
	go func() {
		buf := make([]byte, 1024)
		n, rAddr, err := l.ReadFrom(buf)
		if err != nil {
			return
		}

		pingCh <- buf[:n]
		if _, err := l.WriteTo([]byte("pong"), rAddr); err != nil {
			return
		}
	}()

	pc, err := pcc()
	if err != nil {
		return err
	}
	defer pc.Close()

	go func() {
		if _, err := pc.WriteTo([]byte("ping"), rAddr); err != nil {
			return
		}

		buf := make([]byte, 1024)
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}

		pongCh <- buf[:n]
	}()

	return test(t)
}

type hashPair struct {
	sendHash map[int][]byte
	recvHash map[int][]byte
}

func testLargeDataWithConn(t *testing.T, port uint16, cc func() (net.Conn, error)) error {
	l, err := listen("tcp", ":"+F.ToString(port))
	require.NoError(t, err)
	defer l.Close()

	times := 100
	chunkSize := int64(64 * 1024)

	pingCh, pongCh, test := newLargeDataPair()
	writeRandData := func(conn net.Conn) (map[int][]byte, error) {
		buf := make([]byte, chunkSize)
		hashMap := map[int][]byte{}
		for i := 0; i < times; i++ {
			if _, err := rand.Read(buf[1:]); err != nil {
				return nil, err
			}
			buf[0] = byte(i)

			hash := md5.Sum(buf)
			hashMap[i] = hash[:]

			if _, err := conn.Write(buf); err != nil {
				return nil, err
			}
		}

		return hashMap, nil
	}

	c, err := cc()
	if err != nil {
		return err
	}
	defer c.Close()

	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()

		hashMap := map[int][]byte{}
		buf := make([]byte, chunkSize)

		for i := 0; i < times; i++ {
			_, err := io.ReadFull(c, buf)
			if err != nil {
				t.Log(err.Error())
				return
			}

			hash := md5.Sum(buf)
			hashMap[int(buf[0])] = hash[:]
		}

		sendHash, err := writeRandData(c)
		if err != nil {
			t.Log(err.Error())
			return
		}

		pingCh <- hashPair{
			sendHash: sendHash,
			recvHash: hashMap,
		}
	}()

	go func() {
		sendHash, err := writeRandData(c)
		if err != nil {
			t.Log(err.Error())
			return
		}

		hashMap := map[int][]byte{}
		buf := make([]byte, chunkSize)

		for i := 0; i < times; i++ {
			_, err := io.ReadFull(c, buf)
			if err != nil {
				t.Log(err.Error())
				return
			}

			hash := md5.Sum(buf)
			hashMap[int(buf[0])] = hash[:]
		}

		pongCh <- hashPair{
			sendHash: sendHash,
			recvHash: hashMap,
		}
	}()

	return test(t)
}

func testLargeDataWithPacketConn(t *testing.T, port uint16, pcc func() (net.PacketConn, error)) error {
	return testLargeDataWithPacketConnSize(t, port, 1500, pcc)
}

func testLargeDataWithPacketConnSize(t *testing.T, port uint16, chunkSize int, pcc func() (net.PacketConn, error)) error {
	l, err := listenPacket("udp", ":"+F.ToString(port))
	if err != nil {
		return err
	}
	defer l.Close()

	rAddr := &net.UDPAddr{IP: localIP.AsSlice(), Port: int(port)}

	times := 50

	pingCh, pongCh, test := newLargeDataPair()
	writeRandData := func(pc net.PacketConn, addr net.Addr) (map[int][]byte, error) {
		hashMap := map[int][]byte{}
		mux := sync.Mutex{}
		for i := 0; i < times; i++ {
			buf := make([]byte, chunkSize)
			if _, err := rand.Read(buf[1:]); err != nil {
				t.Log(err.Error())
				continue
			}
			buf[0] = byte(i)

			hash := md5.Sum(buf)
			mux.Lock()
			hashMap[i] = hash[:]
			mux.Unlock()

			if _, err := pc.WriteTo(buf, addr); err != nil {
				t.Log(err.Error())
			}

			time.Sleep(10 * time.Millisecond)
		}

		return hashMap, nil
	}

	go func() {
		var rAddr net.Addr
		hashMap := map[int][]byte{}
		buf := make([]byte, 64*1024)

		for i := 0; i < times; i++ {
			_, rAddr, err = l.ReadFrom(buf)
			if err != nil {
				t.Log(err.Error())
				return
			}
			hash := md5.Sum(buf[:chunkSize])
			hashMap[int(buf[0])] = hash[:]
		}
		sendHash, err := writeRandData(l, rAddr)
		if err != nil {
			t.Log(err.Error())
			return
		}

		pingCh <- hashPair{
			sendHash: sendHash,
			recvHash: hashMap,
		}
	}()

	pc, err := pcc()
	if err != nil {
		return err
	}
	defer pc.Close()

	go func() {
		sendHash, err := writeRandData(pc, rAddr)
		if err != nil {
			t.Log(err.Error())
			return
		}

		hashMap := map[int][]byte{}
		buf := make([]byte, 64*1024)

		for i := 0; i < times; i++ {
			_, _, err := pc.ReadFrom(buf)
			if err != nil {
				t.Log(err.Error())
				return
			}

			hash := md5.Sum(buf[:chunkSize])
			hashMap[int(buf[0])] = hash[:]
		}

		pongCh <- hashPair{
			sendHash: sendHash,
			recvHash: hashMap,
		}
	}()

	return test(t)
}

func testPacketConnTimeout(t *testing.T, pcc func() (net.PacketConn, error)) error {
	pc, err := pcc()
	if err != nil {
		return err
	}

	err = pc.SetReadDeadline(time.Now().Add(time.Millisecond * 300))
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		_, _, err := pc.ReadFrom(buf)
		errCh <- err
	}()

	select {
	case <-errCh:
		return nil
	case <-time.After(time.Second * 10):
		return errors.New("timeout")
	}
}

func listen(network, address string) (net.Listener, error) {
	var lc net.ListenConfig
	lc.Control = control.ReuseAddr()
	var lastErr error
	for i := 0; i < 5; i++ {
		l, err := lc.Listen(context.Background(), network, address)
		if err == nil {
			return l, nil
		}

		lastErr = err
		time.Sleep(5 * time.Millisecond)
	}
	return nil, lastErr
}

func listenPacket(network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	lc.Control = control.ReuseAddr()
	var lastErr error
	for i := 0; i < 5; i++ {
		l, err := lc.ListenPacket(context.Background(), network, address)
		if err == nil {
			return l, nil
		}

		lastErr = err
		time.Sleep(5 * time.Millisecond)
	}
	return nil, lastErr
}

//go:build with_gvisor

package tailscale

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/experimental/locale"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/tailscale/ipn"
	"github.com/sagernet/tailscale/tailcfg"
)

func (t *Endpoint) SendTaildropFile(ctx context.Context, peerStableID string, fileName string, size int64, content io.Reader, progress func(sentBytes int64)) error {
	if !t.started.Load() {
		return E.New("Tailscale is not ready yet")
	}
	err := validateTaildropFileName(fileName)
	if err != nil {
		return err
	}
	localBackend := t.server.ExportLocalBackend()
	if localBackend.State() != ipn.Running {
		return E.New("taildrop: not connected to the tailnet")
	}
	nodeBackend := localBackend.NodeBackend()
	self := nodeBackend.Self()
	if !self.Valid() {
		return E.New("taildrop: not connected to the tailnet")
	}
	if !self.CapMap().Contains(tailcfg.CapabilityFileSharing) {
		return E.New("taildrop: file sharing not enabled by Tailscale admin")
	}
	var peer tailcfg.NodeView
	for _, candidate := range nodeBackend.Peers() {
		if string(candidate.StableID()) == peerStableID {
			peer = candidate
			break
		}
	}
	if !peer.Valid() {
		return E.New("taildrop: peer not found: ", peerStableID)
	}
	if peer.Hostinfo().OS() == "tvOS" {
		return E.New("taildrop: peer cannot receive files")
	}
	if self.User() != peer.User() && !nodeBackend.PeerHasCap(peer, tailcfg.PeerCapabilityFileSharingTarget) {
		return E.New("taildrop: peer is not a permitted file target")
	}
	peerAPIBase := nodeBackend.PeerAPIBase(peer)
	if peerAPIBase == "" {
		return E.New("taildrop: peer does not support peer API")
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				return t.DialContext(ctx, N.NetworkTCP, M.ParseSocksaddr(address))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	fileURL := peerAPIBase + "/v0/put/" + url.PathEscape(fileName)
	offset, remaining := taildropResume(ctx, httpClient, fileURL, content)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, fileURL, &taildropProgressReader{
		reader:   remaining,
		sent:     offset,
		progress: progress,
	})
	if err != nil {
		return err
	}
	if size >= 0 {
		request.ContentLength = size - offset
	}
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	if progress != nil {
		progress(offset)
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return E.Cause(err, "taildrop: send ", fileName)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		messageBytes, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		message := strings.TrimSpace(string(messageBytes))
		if message == errTaildropCanceled.Error() {
			return E.New(fmt.Sprintf(locale.Current().TaildropSendCanceled, fileName))
		}
		return E.New("taildrop: send ", fileName, ": peer responded ", response.Status, ": ", message)
	}
	_, err = io.Copy(io.Discard, response.Body)
	if err != nil {
		return err
	}
	return nil
}

func taildropResume(ctx context.Context, httpClient *http.Client, fileURL string, content io.Reader) (int64, io.Reader) {
	probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
	defer cancelProbe()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, fileURL, nil)
	if err != nil {
		return 0, content
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return 0, content
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, content
	}
	decoder := json.NewDecoder(response.Body)
	var offset int64
	block := make([]byte, 0, taildropBlockSize)
	for {
		var remoteChecksum taildropBlockChecksum
		err = decoder.Decode(&remoteChecksum)
		if err != nil || remoteChecksum.Algorithm != "sha256" || remoteChecksum.Size < 0 || remoteChecksum.Size > taildropBlockSize {
			break
		}
		var n int
		n, err = io.ReadFull(content, block[:remoteChecksum.Size])
		block = block[:n]
		if n == 0 || (err != nil && err != io.EOF && err != io.ErrUnexpectedEOF) {
			break
		}
		localSum := sha256.Sum256(block)
		if hex.EncodeToString(localSum[:]) != remoteChecksum.Checksum {
			break
		}
		offset += int64(n)
		block = block[:0]
	}
	if len(block) > 0 {
		return offset, io.MultiReader(bytes.NewReader(block), content)
	}
	return offset, content
}

type taildropProgressReader struct {
	reader   io.Reader
	sent     int64
	progress func(sentBytes int64)
}

func (r *taildropProgressReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	if n > 0 {
		r.sent += int64(n)
		if r.progress != nil {
			r.progress(r.sent)
		}
	}
	return n, err
}

func (t *Endpoint) taildropTargets() (canShareFiles bool, targets map[string]bool) {
	if !t.started.Load() {
		return false, nil
	}
	localBackend := t.server.ExportLocalBackend()
	if localBackend.State() != ipn.Running {
		return false, nil
	}
	nodeBackend := localBackend.NodeBackend()
	self := nodeBackend.Self()
	if !self.Valid() || !self.CapMap().Contains(tailcfg.CapabilityFileSharing) {
		return false, nil
	}
	targets = make(map[string]bool)
	for _, peer := range nodeBackend.Peers() {
		if !peer.Valid() || peer.Hostinfo().OS() == "tvOS" {
			continue
		}
		if self.User() != peer.User() && !nodeBackend.PeerHasCap(peer, tailcfg.PeerCapabilityFileSharingTarget) {
			continue
		}
		if !nodeBackend.PeerHasPeerAPI(peer) {
			continue
		}
		targets[string(peer.StableID())] = true
	}
	return true, targets
}

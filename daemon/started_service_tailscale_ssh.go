package daemon

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"
)

type windowChangeRequest struct {
	Columns      uint32
	Rows         uint32
	WidthPixels  uint32
	HeightPixels uint32
}

func (s *StartedService) StartTailscaleSSHSession(
	server grpc.BidiStreamingServer[TailscaleSSHClientMessage, TailscaleSSHServerMessage],
) error {
	ctx := server.Context()
	err := s.waitForStarted(ctx)
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	firstMessage, err := server.Recv()
	if err != nil {
		return err
	}
	start := firstMessage.GetStart()

	hostKeys := make([]ssh.PublicKey, 0, len(start.HostKeys))
	for _, line := range start.HostKeys {
		key, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(line))
		if parseErr != nil {
			return E.Cause(parseErr, "parse host key")
		}
		hostKeys = append(hostKeys, key)
	}

	endpoint, err := resolveTailscaleEndpoint(boxService, start.EndpointTag)
	if err != nil {
		return err
	}

	peerAddr := M.ParseSocksaddrHostPort(start.PeerAddress, 22)

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var sendAccess sync.Mutex
	sendMessage := func(msg *TailscaleSSHServerMessage) {
		sendAccess.Lock()
		defer sendAccess.Unlock()
		sendErr := server.Send(msg)
		if sendErr != nil {
			cancel()
		}
	}

	finishWithError := func(message string) error {
		sendMessage(&TailscaleSSHServerMessage{
			Message: &TailscaleSSHServerMessage_Error{Error: &TailscaleSSHError{Message: message}},
		})
		return nil
	}

	peerConn, err := endpoint.DialContext(ctx, N.NetworkTCP, peerAddr)
	if err != nil {
		return finishWithError(E.Cause(err, "dial peer").Error())
	}

	var lastBanner string
	config := &ssh.ClientConfig{
		User: start.Username,
		Auth: nil,
		BannerCallback: func(message string) error {
			lastBanner = message
			sendMessage(&TailscaleSSHServerMessage{
				Message: &TailscaleSSHServerMessage_AuthBanner{
					AuthBanner: &TailscaleSSHAuthBanner{Message: message},
				},
			})
			return nil
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			serverKey := key.Marshal()
			for _, hostKey := range hostKeys {
				if bytes.Equal(serverKey, hostKey.Marshal()) {
					return nil
				}
			}
			return E.New("untrusted host key: ", key.Type())
		},
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(peerConn, peerAddr.String(), config)
	if err != nil {
		common.Close(peerConn)
		banner := strings.TrimSpace(lastBanner)
		if banner != "" {
			return finishWithError(banner)
		}
		return finishWithError(E.Cause(err, "ssh handshake").Error())
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)

	if start.ForwardAgent {
		agentChannels := sshClient.HandleChannelOpen("auth-agent@openssh.com")
		if agentChannels != nil {
			go func() {
				for newChannel := range agentChannels {
					channel, reqs, acceptErr := newChannel.Accept()
					if acceptErr != nil {
						continue
					}
					go ssh.DiscardRequests(reqs)
					go s.forwardSSHAgentChannel(channel)
				}
			}()
		}
	}

	sshSession, err := sshClient.NewSession()
	if err != nil {
		common.Close(sshClient)
		return finishWithError(E.Cause(err, "open ssh session").Error())
	}

	cols := int(start.Columns)
	rows := int(start.Rows)
	err = sshSession.RequestPty(start.TerminalType, rows, cols, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.ECHOE:         1,
		ssh.ECHOK:         1,
		ssh.ECHOKE:        1,
		ssh.ECHOCTL:       1,
		ssh.ICANON:        1,
		ssh.ISIG:          1,
		ssh.IEXTEN:        1,
		ssh.ICRNL:         1,
		ssh.IXON:          1,
		ssh.IXANY:         1,
		ssh.IMAXBEL:       1,
		ssh.OPOST:         1,
		ssh.ONLCR:         1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	})
	if err != nil {
		common.Close(sshSession, sshClient)
		return finishWithError(E.Cause(err, "request pty").Error())
	}
	if start.WidthPixels > 0 || start.HeightPixels > 0 {
		_, _ = sshSession.SendRequest("window-change", false, ssh.Marshal(&windowChangeRequest{
			Columns:      uint32(start.Columns),
			Rows:         uint32(start.Rows),
			WidthPixels:  uint32(start.WidthPixels),
			HeightPixels: uint32(start.HeightPixels),
		}))
	}

	if start.ForwardAgent {
		err = agent.RequestAgentForwarding(sshSession)
		if err != nil {
			common.Close(sshSession, sshClient)
			return finishWithError(E.Cause(err, "request agent forwarding").Error())
		}
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		common.Close(sshSession, sshClient)
		return finishWithError(E.Cause(err, "stdin pipe").Error())
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		common.Close(sshSession, sshClient)
		return finishWithError(E.Cause(err, "stdout pipe").Error())
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		common.Close(sshSession, sshClient)
		return finishWithError(E.Cause(err, "stderr pipe").Error())
	}
	err = sshSession.Shell()
	if err != nil {
		common.Close(sshSession, sshClient)
		return finishWithError(E.Cause(err, "start shell").Error())
	}

	var workersWg sync.WaitGroup

	sendMessage(&TailscaleSSHServerMessage{
		Message: &TailscaleSSHServerMessage_Ready{Ready: &TailscaleSSHReady{}},
	})

	workersWg.Add(1)
	go func() {
		defer workersWg.Done()
		for {
			msg, recvErr := server.Recv()
			if recvErr == io.EOF {
				stdin.Close()
				return
			}
			if recvErr != nil {
				cancel()
				return
			}
			switch m := msg.GetMessage().(type) {
			case *TailscaleSSHClientMessage_Input:
				if len(m.Input.Data) == 0 {
					continue
				}
				_, writeErr := stdin.Write(m.Input.Data)
				if writeErr != nil {
					cancel()
					return
				}
			case *TailscaleSSHClientMessage_Resize:
				_, _ = sshSession.SendRequest("window-change", false, ssh.Marshal(&windowChangeRequest{
					Columns:      uint32(m.Resize.Columns),
					Rows:         uint32(m.Resize.Rows),
					WidthPixels:  uint32(m.Resize.WidthPixels),
					HeightPixels: uint32(m.Resize.HeightPixels),
				}))
			}
		}
	}()

	pumpReader := func(reader io.Reader) {
		defer workersWg.Done()
		buffer := buf.Get(buf.BufferSize)
		defer buf.Put(buffer)
		for {
			n, readErr := reader.Read(buffer)
			if n > 0 {
				sendMessage(&TailscaleSSHServerMessage{
					Message: &TailscaleSSHServerMessage_Output{Output: &TailscaleSSHOutput{Data: bytes.Clone(buffer[:n])}},
				})
			}
			if readErr != nil {
				return
			}
		}
	}
	workersWg.Add(1)
	go pumpReader(stdout)
	workersWg.Add(1)
	go pumpReader(stderr)

	workersWg.Add(1)
	go func() {
		defer workersWg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				_, _, keepAliveErr := sshConn.SendRequest("keepalive@openssh.com", true, nil)
				if keepAliveErr != nil {
					cancel()
					return
				}
			}
		}
	}()

	workersWg.Add(1)
	go func() {
		defer workersWg.Done()
		waitErr := sshSession.Wait()
		exitMessage := &TailscaleSSHExit{}
		switch waitErrTyped := waitErr.(type) {
		case nil:
		case *ssh.ExitError:
			exitMessage.ExitCode = int32(waitErrTyped.ExitStatus())
			exitMessage.Signal = waitErrTyped.Signal()
		default:
			exitMessage.ErrorMessage = waitErrTyped.Error()
		}
		sendMessage(&TailscaleSSHServerMessage{
			Message: &TailscaleSSHServerMessage_Exit{Exit: exitMessage},
		})
		cancel()
	}()

	go func() {
		<-sessionCtx.Done()
		common.Close(peerConn, sshSession, sshClient)
	}()

	workersWg.Wait()
	return nil
}

func (s *StartedService) forwardSSHAgentChannel(channel ssh.Channel) {
	defer channel.Close()
	fd, err := s.handler.ConnectSSHAgent()
	if err != nil {
		return
	}
	file := os.NewFile(uintptr(fd), "ssh-agent")
	conn, err := net.FileConn(file)
	file.Close()
	if err != nil {
		return
	}
	defer conn.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(conn, channel)
	}()
	go func() {
		defer wg.Done()
		io.Copy(channel, conn)
	}()
	wg.Wait()
}

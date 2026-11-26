//go:build with_tailscale

package tailssh

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service/filemanager"
	"github.com/sagernet/tailscale/ipn/ipnlocal"
	tsDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/tailcfg"
	"github.com/sagernet/tailscale/tailcfg/nodecap"
	"github.com/sagernet/tailscale/tsnet"
	"github.com/sagernet/tailscale/wgengine/router"
	"github.com/sagernet/tailscale/wgengine/wgcfg"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

type sshConnContextKey struct{}

type Server struct {
	tsnetServer       *tsnet.Server
	platformInterface adapter.PlatformInterface
	logger            logger.ContextLogger
	listener          net.Listener
	server            *gliderssh.Server
	backend           shellBackend

	hostSigner gossh.Signer

	disablePTY        bool
	disableSFTP       bool
	disableForwarding bool

	serverCtx    context.Context
	serverCancel context.CancelFunc

	access    sync.Mutex
	closed    bool
	conns     map[*sshConn]struct{}
	policy    *tailcfg.SSHPolicy
	sessionWg sync.WaitGroup
}

func New(ctx context.Context, tsnetServer *tsnet.Server, platformInterface adapter.PlatformInterface, options *option.TailscaleSSHServerOptions, logger logger.ContextLogger) (*Server, error) {
	s := &Server{
		tsnetServer:       tsnetServer,
		platformInterface: platformInterface,
		logger:            logger,
		disablePTY:        options.DisablePTY,
		disableSFTP:       options.DisableSFTP,
		disableForwarding: options.DisableForwarding,
		conns:             make(map[*sshConn]struct{}),
	}
	s.serverCtx, s.serverCancel = context.WithCancel(ctx)
	hostSigner, err := s.loadOrGenerateHostKey()
	if err != nil {
		return nil, err
	}
	s.hostSigner = hostSigner
	s.backend = selectShellBackend(platformInterface)
	return s, nil
}

func (s *Server) loadOrGenerateHostKey() (gossh.Signer, error) {
	if s.platformInterface != nil {
		keyData, err := s.platformInterface.ReadSystemSSHHostKey()
		if err == nil {
			signer, parseErr := gossh.ParsePrivateKey(keyData)
			if parseErr == nil {
				s.logger.Debug("loaded SSH host key via platform")
				return signer, nil
			}
			s.logger.Warn("failed to parse SSH host key from platform: ", parseErr)
		}
	}
	// Read the system host key when privileged, but never write back to it: the
	// generated key below always goes to the tsnet directory, so a parse failure
	// can never clobber the operating system's sshd host key.
	if isPrivilegedUser() {
		systemKey := systemHostKeyPath()
		if systemKey != "" {
			keyData, err := filemanager.ReadFile(s.serverCtx, systemKey)
			if err == nil {
				signer, parseErr := gossh.ParsePrivateKey(keyData)
				if parseErr == nil {
					s.logger.Debug("loaded SSH host key from ", systemKey)
					return signer, nil
				}
				s.logger.Warn("failed to parse system SSH host key: ", parseErr)
			}
		}
	}
	keyPath := filepath.Join(s.tsnetServer.Dir, "ssh_host_ed25519_key")
	keyData, err := filemanager.ReadFile(s.serverCtx, keyPath)
	if err == nil {
		signer, parseErr := gossh.ParsePrivateKey(keyData)
		if parseErr == nil {
			s.logger.Debug("loaded SSH host key from ", keyPath)
			return signer, nil
		}
		s.logger.Warn("failed to parse SSH host key, regenerating: ", parseErr)
	}
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	keyBytes, err := gossh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, err
	}
	pemData := pem.EncodeToMemory(keyBytes)
	dir := filepath.Dir(keyPath)
	err = filemanager.MkdirAll(s.serverCtx, dir, 0o700)
	if err != nil {
		return nil, err
	}
	err = filemanager.WriteFile(s.serverCtx, keyPath, pemData, 0o600)
	if err != nil {
		return nil, err
	}
	s.logger.Info("generated SSH host key at ", keyPath)
	return gossh.NewSignerFromKey(privateKey)
}

func (s *Server) Start() error {
	listener, err := s.tsnetServer.Listen("tcp", ":22")
	if err != nil {
		return err
	}
	s.listener = listener
	fwdHandler := &gliderssh.ForwardedTCPHandler{}
	unixFwdHandler := &gliderssh.ForwardedUnixHandler{}
	sshServer := &gliderssh.Server{
		Version:              "sing-box",
		ConnCallback:         s.newConn,
		ServerConfigCallback: s.serverConfig,
		Handler:              s.handleSession,
		SubsystemHandlers: map[string]gliderssh.SubsystemHandler{
			"sftp": s.handleSession,
		},
		ChannelHandlers: map[string]gliderssh.ChannelHandler{
			"direct-tcpip":                   gliderssh.DirectTCPIPHandler,
			"direct-streamlocal@openssh.com": gliderssh.DirectStreamLocalHandler,
		},
		RequestHandlers: map[string]gliderssh.RequestHandler{
			"tcpip-forward":                          fwdHandler.HandleSSHRequest,
			"cancel-tcpip-forward":                   fwdHandler.HandleSSHRequest,
			"streamlocal-forward@openssh.com":        unixFwdHandler.HandleSSHRequest,
			"cancel-streamlocal-forward@openssh.com": unixFwdHandler.HandleSSHRequest,
		},
		LocalPortForwardingCallback:   s.allowLocalForward,
		ReversePortForwardingCallback: s.allowReverseForward,
	}
	if s.disablePTY {
		sshServer.PtyCallback = func(ctx gliderssh.Context, pty gliderssh.Pty) bool {
			return false
		}
	}
	if !s.disableForwarding {
		sshServer.LocalUnixForwardingCallback = s.allowLocalUnixForward
		sshServer.ReverseUnixForwardingCallback = s.allowReverseUnixForward
	}
	maps.Copy(sshServer.RequestHandlers, gliderssh.DefaultRequestHandlers)
	maps.Copy(sshServer.ChannelHandlers, gliderssh.DefaultChannelHandlers)
	maps.Copy(sshServer.SubsystemHandlers, gliderssh.DefaultSubsystemHandlers)
	sshServer.AddHostKey(s.hostSigner)
	s.server = sshServer
	hostKeyPublic := strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s.hostSigner.PublicKey())))
	s.tsnetServer.ExportLocalBackend().SetExternalSSHHostKeys([]string{hostKeyPublic})
	go func() {
		err := sshServer.Serve(listener)
		if err != nil && !errors.Is(err, gliderssh.ErrServerClosed) {
			s.logger.Error("SSH server stopped: ", err)
		}
	}()
	s.logger.Info("SSH server started on :22")
	return nil
}

func (s *Server) Close() error {
	s.serverCancel()
	s.access.Lock()
	s.closed = true
	for conn := range s.conns {
		conn.close()
	}
	s.access.Unlock()
	var err error
	if s.server != nil {
		err = s.server.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	sessionsDone := make(chan struct{})
	go func() {
		s.sessionWg.Wait()
		close(sessionsDone)
	}()
	select {
	case <-sessionsDone:
	case <-time.After(shutdownTimeout):
		s.logger.Warn("SSH sessions did not finish within ", shutdownTimeout, " of shutdown")
	}
	if s.backend != nil {
		s.backend.Close()
	}
	return err
}

func (s *Server) newConn(ctx gliderssh.Context, netConn net.Conn) net.Conn {
	authCtx, authCancel := context.WithCancel(s.serverCtx)
	conn := &sshConn{
		server:     s,
		netConn:    netConn,
		authCtx:    authCtx,
		authCancel: authCancel,
		sessions:   make(map[*activeSession]struct{}),
	}
	conn.access.Lock()
	conn.armLoginGraceLocked(loginGraceTime)
	conn.access.Unlock()
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		conn.stopTimers()
		authCancel()
		return nil
	}
	s.conns[conn] = struct{}{}
	s.access.Unlock()
	ctx.SetValue(sshConnContextKey{}, conn)
	context.AfterFunc(ctx, func() {
		s.access.Lock()
		delete(s.conns, conn)
		s.access.Unlock()
		conn.stopTimers()
		authCancel()
	})
	return netConn
}

func (s *Server) serverConfig(ctx gliderssh.Context) *gossh.ServerConfig {
	conn := s.connFromContext(ctx)
	var preAuthConn gossh.ServerPreAuthConn
	config := &gossh.ServerConfig{
		AuthLogCallback: func(_ gossh.ConnMetadata, _ string, err error) {
			if err == nil {
				conn.completeAuth()
			}
		},
		PreAuthConnCallback: func(conn gossh.ServerPreAuthConn) {
			preAuthConn = conn
		},
		NoClientAuthCallback: func(metadata gossh.ConnMetadata) (*gossh.Permissions, error) {
			return s.authenticate(conn, preAuthConn, metadata)
		},
		PasswordCallback: func(metadata gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			return s.authenticate(conn, preAuthConn, metadata)
		},
		PublicKeyCallback: func(metadata gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			return s.authenticate(conn, preAuthConn, metadata)
		},
	}
	return config
}

func (s *Server) sshPolicy() *tailcfg.SSHPolicy {
	netMap := s.tsnetServer.ExportLocalBackend().NetMapNoPeers()
	s.access.Lock()
	defer s.access.Unlock()
	if netMap != nil {
		s.policy = netMap.SSHPolicy
	}
	return s.policy
}

func (s *Server) authenticate(conn *sshConn, preAuthConn gossh.ServerPreAuthConn, metadata gossh.ConnMetadata) (*gossh.Permissions, error) {
	if conn.action != nil {
		return &gossh.Permissions{}, nil
	}
	remoteAddrPort := M.AddrPortFromNet(metadata.RemoteAddr())
	node, userProfile, found := s.tsnetServer.ExportLocalBackend().WhoIs("tcp", remoteAddrPort)
	// Every denial returns an empty *gossh.PartialSuccessError so x/crypto/ssh
	// stops offering further auth methods instead of re-running policy
	// evaluation (and hold-and-delegate) once per method.
	if !found {
		s.logger.Warn("SSH auth: unknown peer ", remoteAddrPort)
		return nil, &gossh.PartialSuccessError{}
	}
	conn.node = node
	conn.userProfile = userProfile
	conn.sshUser = metadata.User()
	conn.srcIP = remoteAddrPort.Addr()
	policy := s.sshPolicy()
	if policy == nil {
		s.logger.Warn("SSH auth: no SSH policy")
		return nil, &gossh.PartialSuccessError{}
	}
	match, err := conn.evaluatePolicy(policy, s.lookupUser)
	if err != nil {
		s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.sshUser, ": ", err)
		return nil, &gossh.PartialSuccessError{}
	}
	action := match.action
	conn.action0 = action
	conn.latchedRule = match.rule
	if action.Accept || action.HoldAndDelegate != "" {
		localUser, lookupErr := s.lookupUser(match.localUser)
		if lookupErr != nil {
			s.logger.Info("SSH auth rejected for ", userProfile.LoginName, ": lookup local user ", match.localUser, ": ", lookupErr)
			return nil, &gossh.PartialSuccessError{}
		}
		conn.localUser = localUser
		conn.mappedLocalUser = match.localUser
		conn.acceptEnv = match.acceptEnv
	}
	for hops := 0; ; hops++ {
		if action.Message != "" {
			err = preAuthConn.SendAuthBanner(action.Message)
			if err != nil {
				s.logger.Info("SSH auth: send banner: ", err)
				return nil, &gossh.PartialSuccessError{}
			}
		}
		if action.Reject {
			s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.sshUser)
			return nil, &gossh.PartialSuccessError{}
		}
		if action.HoldAndDelegate == "" {
			break
		}
		if hops >= 10 {
			s.logger.Info("SSH auth rejected: hold-and-delegate chain too long")
			return nil, &gossh.PartialSuccessError{}
		}
		conn.extendLoginGrace(holdDelegateGrace)
		action, err = s.holdAndDelegate(conn, action)
		if err != nil {
			s.logger.Info("SSH auth rejected for ", userProfile.LoginName, ": ", err)
			return nil, &gossh.PartialSuccessError{}
		}
	}
	if !action.Accept {
		s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.sshUser)
		return nil, &gossh.PartialSuccessError{}
	}
	conn.connID = newConnID()
	conn.action = action
	s.logger.Info("SSH auth accepted: ", userProfile.LoginName, " -> ", conn.localUser.Username)
	return &gossh.Permissions{}, nil
}

func (s *Server) holdAndDelegate(conn *sshConn, action *tailcfg.SSHAction) (*tailcfg.SSHAction, error) {
	lb := s.tsnetServer.ExportLocalBackend()
	addr4, addr6 := s.tsnetServer.TailscaleIPs()
	dstNodeIP := addr4
	if !dstNodeIP.IsValid() {
		dstNodeIP = addr6
	}
	srcNodeIP := conn.srcIP
	if !srcNodeIP.IsValid() && conn.node.Addresses().Len() > 0 {
		srcNodeIP = conn.node.Addresses().At(0).Addr()
	}
	var dstNodeID string
	netMap := lb.NetMapNoPeers()
	if netMap != nil && netMap.SelfNode.Valid() {
		dstNodeID = fmt.Sprint(int64(netMap.SelfNode.ID()))
	}
	// Escape interpolated values; $SSH_USER and $LOCAL_USER are client-controlled
	// (mapLocalUser "=" passes the requested name through). Numeric node IDs need
	// no escaping.
	replacer := strings.NewReplacer(
		"$SRC_NODE_IP", url.QueryEscape(srcNodeIP.String()),
		"$SRC_NODE_ID", fmt.Sprint(int64(conn.node.ID())),
		"$DST_NODE_IP", url.QueryEscape(dstNodeIP.String()),
		"$DST_NODE_ID", dstNodeID,
		"$SSH_USER", url.QueryEscape(conn.sshUser),
		"$LOCAL_USER", url.QueryEscape(conn.mappedLocalUser),
	)
	delegateURL := replacer.Replace(action.HoldAndDelegate)
	ctx, cancel := context.WithTimeout(conn.authCtx, delegateFetchTimeout)
	defer cancel()
	for {
		err := ctx.Err()
		if err != nil {
			return nil, err
		}
		newAction, err := s.fetchSSHAction(ctx, lb, delegateURL)
		if err == nil {
			return newAction, nil
		}
		s.logger.Warn("hold and delegate: ", err)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
}

func (s *Server) fetchSSHAction(ctx context.Context, lb *ipnlocal.LocalBackend, delegateURL string) (*tailcfg.SSHAction, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, delegateURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := lb.DoNoiseRequest(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, E.New("unexpected status: ", response.Status)
	}
	var action tailcfg.SSHAction
	err = json.NewDecoder(response.Body).Decode(&action)
	if err != nil {
		return nil, err
	}
	return &action, nil
}

func (s *Server) connFromContext(ctx context.Context) *sshConn {
	return ctx.Value(sshConnContextKey{}).(*sshConn)
}

func (s *Server) handleSession(session gliderssh.Session) {
	conn := s.connFromContext(session.Context())
	_, err := conn.checkPolicy(s.lookupUser)
	if err != nil {
		io.WriteString(session.Stderr(), "tailscale: access revoked\r\n")
		session.Exit(255)
		conn.revoke(err)
		return
	}
	ctx, cancel := context.WithCancelCause(session.Context())
	defer cancel(nil)
	active := &activeSession{cancel: cancel, done: make(chan struct{})}
	if !conn.attachSession(active) {
		session.Exit(255)
		return
	}
	defer conn.detachSession(active)
	if conn.action.SessionDuration != 0 {
		timer := time.AfterFunc(conn.action.SessionDuration, func() {
			cancel(&terminationError{message: "Session duration exceeded."})
		})
		defer timer.Stop()
	}
	subsystem := session.Subsystem()
	if subsystem == "sftp" {
		s.handleSFTP(ctx, cancel, session, conn)
		return
	}
	if subsystem != "" {
		fmt.Fprintf(session.Stderr(), "unsupported subsystem: %s\r\n", subsystem)
		session.Exit(1)
		return
	}
	localUser := conn.localUser
	err = verifyShellIdentity(s.platformInterface, localUser)
	if err != nil {
		s.logger.Warn("shell rejected for ", localUser.Username, ": ", err)
		fmt.Fprintf(session.Stderr(), "%s\r\n", err)
		session.Exit(1)
		return
	}
	var agentSocketPath string
	if conn.action.AllowAgentForwarding && !s.disableForwarding && gliderssh.AgentRequested(session) {
		agentListener, err := newAgentListener(localUser)
		if err == nil {
			defer agentListener.Close()
			agentSocketPath = agentListener.Addr().String()
			go gliderssh.ForwardAgentConnections(agentListener, session)
		} else {
			s.logger.Warn("create agent listener: ", err)
		}
	}
	env := s.buildEnvironment(session, conn)
	if agentSocketPath != "" {
		env = append(env, "SSH_AUTH_SOCK="+agentSocketPath)
	}
	ptyReq, winCh, isPty := session.Pty()
	session.DisablePTYEmulation()
	command := session.RawCommand()
	var term string
	var rows, cols, widthPixels, heightPixels uint16
	if isPty {
		term = ptyReq.Term
		rows = clampWindowDimension(ptyReq.Window.Height)
		cols = clampWindowDimension(ptyReq.Window.Width)
		widthPixels = clampWindowDimension(ptyReq.Window.WidthPixels)
		heightPixels = clampWindowDimension(ptyReq.Window.HeightPixels)
	}
	rec, err := s.startSessionRecording(ctx, cancel, session, conn)
	if err != nil {
		return
	}
	if rec != nil {
		defer rec.Close()
		defer cancel(nil)
	}
	shellSession, err := s.backend.OpenSession(shellRequest{
		User:         localUser,
		Command:      command,
		Env:          env,
		Term:         term,
		Rows:         rows,
		Cols:         cols,
		WidthPixels:  widthPixels,
		HeightPixels: heightPixels,
	})
	if err != nil {
		s.logger.Error("failed to open shell session: ", err)
		fmt.Fprintf(session.Stderr(), "failed to open shell: %s\r\n", err)
		session.Exit(1)
		return
	}
	var shellAccess sync.Mutex
	shellAlive := true
	// Buffer to gliderssh's maxSigBufSize so the goroutine it spawns to replay
	// buffered signals (one unconditional blocking send per signal) can never wedge
	// if this connection ends before the drain goroutine consumes them all.
	sigCh := make(chan gliderssh.Signal, 128)
	session.Signals(sigCh)
	// gliderssh delivers signals synchronously from its single per-session request
	// loop while holding the session lock; an undrained sigCh blocks that loop and
	// deadlocks Exit, which needs the same lock. Drain for the whole connection
	// lifetime; sigCh is never closed by gliderssh, so stop on the connection context.
	go func() {
		for {
			select {
			case <-session.Context().Done():
				return
			case sig := <-sigCh:
				sysSig := sshSignalToSyscall(sig)
				if sysSig == 0 {
					continue
				}
				shellAccess.Lock()
				if shellAlive {
					shellSession.Signal(sysSig)
				}
				shellAccess.Unlock()
			}
		}
	}()
	if isPty && winCh != nil {
		// winCh (buffer 1) is fed synchronously from the same request loop and closed
		// by gliderssh when the loop ends. Drain to completion: stopping early blocks
		// the loop on a full winCh and leaks its goroutine.
		go func() {
			for win := range winCh {
				shellAccess.Lock()
				if shellAlive {
					shellSession.Resize(
						clampWindowDimension(win.Height),
						clampWindowDimension(win.Width),
						clampWindowDimension(win.WidthPixels),
						clampWindowDimension(win.HeightPixels),
					)
				}
				shellAccess.Unlock()
			}
		}()
	}
	s.pumpSession(ctx, session, shellSession, rec)
	// Mark the shell closed under shellAccess so the drain goroutines never touch it
	// after Close (Windows process-handle use-after-close, pty fd resize race), then
	// close. The goroutines keep draining their gliderssh-owned channels until the
	// request loop ends (winCh close) and the connection closes (session context done).
	shellAccess.Lock()
	shellAlive = false
	shellSession.Close()
	shellAccess.Unlock()
}

func (s *Server) startSessionRecording(ctx context.Context, cancel context.CancelCauseFunc, session gliderssh.Session, conn *sshConn) (*recording, error) {
	recorderList, onFailure := recordingConfig(conn.action0, conn.action)
	if len(recorderList) == 0 {
		return nil, nil
	}
	rec, err := s.startNewRecording(ctx, cancel, session, conn, recorderList, onFailure)
	if err != nil {
		var rejected *recordingRejectedError
		if errors.As(err, &rejected) && rejected.message != "" {
			io.WriteString(session.Stderr(), rejected.message+"\r\n")
		} else {
			io.WriteString(session.Stderr(), "can't start new recording\r\n")
		}
		s.logger.Error("recording: ", err)
		session.Exit(254)
		return nil, err
	}
	return rec, nil
}

// pumpSession copies between the SSH channel and the backend session. It signals
// stdin EOF to the child (without killing it) when the client closes its input,
// and waits for all output to drain before reporting the exit status, because
// gliderssh closes the channel immediately after Exit returns.
func (s *Server) pumpSession(ctx context.Context, session gliderssh.Session, shell shellSession, rec *recording) {
	go func() {
		io.Copy(shell, session)
		shell.CloseWrite()
	}()
	outputDone := make(chan struct{})
	go func() {
		io.Copy(rec.writer(session), shell)
		close(outputDone)
	}()
	exitCh := make(chan uint32, 1)
	go func() {
		exitStatus, err := shell.Wait()
		if err != nil {
			s.logger.Error("wait session: ", err)
			exitStatus = 1
		}
		exitCh <- exitStatus
	}()
	select {
	case <-ctx.Done():
		termErr, isTermination := errors.AsType[*terminationError](context.Cause(ctx))
		if isTermination {
			io.WriteString(session.Stderr(), "\r\n\r\n"+termErr.message+"\r\n\r\n")
		}
		session.Exit(130)
	case exitStatus := <-exitCh:
		select {
		case <-outputDone:
		case <-ctx.Done():
		case <-time.After(ioDrainTimeout):
			s.logger.Debug("session output did not drain within ", ioDrainTimeout)
		}
		session.Exit(int(exitStatus))
	}
}

func (s *Server) handleSFTP(ctx context.Context, cancel context.CancelCauseFunc, session gliderssh.Session, conn *sshConn) {
	if s.disableSFTP {
		fmt.Fprint(session.Stderr(), "SFTP is disabled.\r\n")
		session.Exit(1)
		return
	}
	rec, err := s.startSessionRecording(ctx, cancel, session, conn)
	if err != nil {
		return
	}
	if rec != nil {
		defer rec.Close()
		defer cancel(nil)
	}
	localUser := conn.localUser
	sftpPath, err := lookupSFTPServer(s.platformInterface)
	if err != nil {
		match, matchErr := requestedUserMatchesProcess(localUser)
		if matchErr != nil {
			s.logger.Warn("builtin sftp rejected for ", localUser.Username, ": ", matchErr)
			fmt.Fprint(session.Stderr(), "SFTP unavailable: builtin server cannot impersonate a different local user.\r\n")
			session.Exit(1)
			return
		}
		if !match {
			s.logger.Warn("builtin sftp rejected for ", localUser.Username, ": running process identity differs from requested user")
			fmt.Fprint(session.Stderr(), "SFTP unavailable: builtin server cannot impersonate a different local user.\r\n")
			session.Exit(1)
			return
		}
		s.logger.Debug("sftp-server not found, using builtin: ", err)
		s.serveBuiltinSFTP(ctx, session, localUser)
		return
	}
	err = verifyShellIdentity(s.platformInterface, localUser)
	if err != nil {
		s.logger.Warn("sftp rejected for ", localUser.Username, ": ", err)
		fmt.Fprintf(session.Stderr(), "%s\r\n", err)
		session.Exit(1)
		return
	}
	env := s.buildEnvironment(session, conn)
	sftpSession, err := s.backend.OpenSession(shellRequest{
		User:    localUser,
		Command: sftpCommand(sftpPath, localUser.Shell),
		Env:     env,
	})
	if err != nil {
		s.logger.Error("failed to start sftp-server: ", err)
		fmt.Fprintf(session.Stderr(), "failed to start SFTP: %s\r\n", err)
		session.Exit(1)
		return
	}
	// Use the cancelable child ctx (not session.Context()) so SessionDuration and
	// OnReconfig revocation also terminate SFTP transfers.
	s.pumpSession(ctx, session, sftpSession, nil)
	sftpSession.Close()
}

func (s *Server) serveBuiltinSFTP(ctx context.Context, session gliderssh.Session, user *adapter.PlatformUser) {
	// The builtin server runs in-process with no chroot/jail; WithServerWorkingDirectory
	// only sets a default for relative paths, so absolute paths are unconfined. The
	// caller only reaches here when the target user matches the process identity, so
	// this grants no access beyond what the running process already has.
	var opts []sftp.ServerOption
	if user != nil && user.HomeDir != "" {
		opts = append(opts, sftp.WithServerWorkingDirectory(user.HomeDir))
	}
	server, err := sftp.NewServer(session, opts...)
	if err != nil {
		s.logger.Error("create builtin sftp server: ", err)
		fmt.Fprintf(session.Stderr(), "failed to start SFTP: %s\r\n", err)
		session.Exit(1)
		return
	}
	defer server.Close()
	// Terminate the transfer when the session ctx is cancelled (SessionDuration
	// elapsed or OnReconfig revoked access): closing the SSH channel unblocks Serve.
	stop := context.AfterFunc(ctx, func() {
		session.Close()
	})
	defer stop()
	err = server.Serve()
	if err != nil && !errors.Is(err, io.EOF) {
		s.logger.Error("builtin sftp serve: ", err)
		session.Exit(1)
		return
	}
	session.Exit(0)
}

func (s *Server) buildEnvironment(session gliderssh.Session, conn *sshConn) []string {
	localUser := conn.localUser
	env := []string{
		"USER=" + localUser.Username,
		"LOGNAME=" + localUser.Username,
		"HOME=" + localUser.HomeDir,
		"SHELL=" + localUser.Shell,
	}
	defaultPath := defaultPathEnv(s.platformInterface)
	if defaultPath != "" {
		env = append(env, "PATH="+defaultPath)
	}
	env = append(env, platformEnvironment(localUser)...)
	remoteAddr := session.RemoteAddr()
	localAddr := session.LocalAddr()
	if remoteAddr != nil && localAddr != nil {
		remoteHost, remotePort, _ := net.SplitHostPort(remoteAddr.String())
		localHost, localPort, _ := net.SplitHostPort(localAddr.String())
		env = append(env,
			"SSH_CLIENT="+remoteHost+" "+remotePort+" "+localPort,
			"SSH_CONNECTION="+remoteHost+" "+remotePort+" "+localHost+" "+localPort,
		)
	}
	ptyReq, _, isPty := session.Pty()
	if isPty {
		env = append(env, "TERM="+ptyReq.Term)
	}
	// Only honor the rule's AcceptEnv patterns when the node has the ssh-env-vars
	// capability, matching upstream's capability gate.
	acceptEnv := conn.acceptEnv
	if len(acceptEnv) > 0 {
		netMap := s.tsnetServer.ExportLocalBackend().NetMapNoPeers()
		if netMap == nil || !netMap.HasCap(nodecap.SSHEnvironmentVariables) {
			acceptEnv = nil
		}
	}
	for _, clientEnv := range session.Environ() {
		// TERM is already set authoritatively from the PTY request above; skip a
		// client-sent duplicate that would otherwise override it.
		if isPty && strings.HasPrefix(clientEnv, "TERM=") {
			continue
		}
		if acceptClientEnv(clientEnv, acceptEnv) {
			env = append(env, clientEnv)
		}
	}
	return env
}

// clampWindowDimension maps a client-supplied terminal dimension into uint16 without
// the wraparound a bare cast causes (e.g. 65536 -> 0, a zero-size terminal): values
// outside the range saturate instead.
func clampWindowDimension(value int) uint16 {
	if value < 0 {
		return 0
	}
	if value > 0xffff {
		return 0xffff
	}
	return uint16(value)
}

func (s *Server) allowLocalForward(ctx gliderssh.Context, destinationHost string, destinationPort uint32) bool {
	if s.disableForwarding {
		return false
	}
	conn := s.connFromContext(ctx)
	return conn.action.AllowLocalPortForwarding && conn.recheckPolicy(conn.latchedUser)
}

func (s *Server) allowReverseForward(ctx gliderssh.Context, bindHost string, bindPort uint32) bool {
	if s.disableForwarding {
		return false
	}
	conn := s.connFromContext(ctx)
	if !conn.action.AllowRemotePortForwarding || !conn.recheckPolicy(conn.latchedUser) {
		return false
	}
	if bindHost != "localhost" {
		bindAddr, err := netip.ParseAddr(bindHost)
		if err != nil || !bindAddr.IsLoopback() {
			s.logger.Info("SSH reverse port forward rejected: binding to non-loopback address ", bindHost, " is not permitted")
			return false
		}
	}
	if bindPort != 0 && bindPort < 1024 && privilegedPortRestricted(conn.localUser) {
		s.logger.Info("SSH reverse port forward rejected: binding privileged port ", bindPort, " as ", conn.localUser.Username, " is not permitted")
		return false
	}
	return true
}

func (s *Server) allowLocalUnixForward(ctx gliderssh.Context, socketPath string) (net.Conn, error) {
	if s.disableForwarding {
		return nil, gliderssh.ErrRejected
	}
	conn := s.connFromContext(ctx)
	if !conn.action.AllowLocalPortForwarding || !conn.recheckPolicy(conn.latchedUser) {
		return nil, gliderssh.ErrRejected
	}
	opts := gliderssh.UnixForwardingOptions{
		AllowedDirectories: userSocketDirectories(conn.localUser),
	}
	return gliderssh.NewLocalUnixForwardingCallback(opts)(ctx, socketPath)
}

func (s *Server) allowReverseUnixForward(ctx gliderssh.Context, socketPath string) (net.Listener, error) {
	if s.disableForwarding {
		return nil, gliderssh.ErrRejected
	}
	conn := s.connFromContext(ctx)
	if !conn.action.AllowRemotePortForwarding || !conn.recheckPolicy(conn.latchedUser) {
		return nil, gliderssh.ErrRejected
	}
	opts := gliderssh.UnixForwardingOptions{
		AllowedDirectories: userSocketDirectories(conn.localUser),
		BindUnlink:         true,
	}
	listener, err := gliderssh.NewReverseUnixForwardingCallback(opts)(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	err = chownUserSocket(listener, conn.localUser)
	if err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func (s *Server) OnReconfig(cfg *wgcfg.Config, routerCfg *router.Config, dnsCfg *tsDNS.Config) {
	if s.tsnetServer.ExportLocalBackend().NetMapNoPeers() == nil {
		return
	}
	s.access.Lock()
	conns := slices.Collect(maps.Keys(s.conns))
	s.access.Unlock()
	for _, conn := range conns {
		if conn.authCompleted.Load() {
			go conn.recheckPolicy(s.lookupUser)
		}
	}
}

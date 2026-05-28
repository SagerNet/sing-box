//go:build with_gvisor

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
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gliderssh "github.com/sagernet/gliderssh"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	tsDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/tailcfg"
	"github.com/sagernet/tailscale/tsnet"
	"github.com/sagernet/tailscale/wgengine/router"
	"github.com/sagernet/tailscale/wgengine/wgcfg"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

type sshConnContextKey struct{}

type sshConnInfo struct {
	node        tailcfg.NodeView
	userProfile tailcfg.UserProfile
	sshUser     string
	srcIP       netip.Addr
	localUser   string
	action      *tailcfg.SSHAction
	acceptEnv   []string

	// action0 is the initially matched rule's action, retained so session
	// recording can fall back to its recorders when a hold-and-delegate result
	// (which replaces action) carries none. connID is shared with control and
	// reused across multiplexed sessions on this connection.
	action0 *tailcfg.SSHAction
	connID  string

	// localUser is fixed for the lifetime of an accepted connection, so the OS
	// lookup is resolved once and memoized here for all sessions/forwards.
	localUserOnce sync.Once
	localUserInfo *adapter.PlatformUser
	localUserErr  error
}

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

	done         chan struct{}
	serverCtx    context.Context
	serverCancel context.CancelFunc

	access      sync.Mutex
	activeConns map[*activeSession]struct{}
	sessionWg   sync.WaitGroup
}

// activeSession is the map key for activeConns so that multiple concurrent
// sessions sharing one *sshConnInfo are tracked and revoked independently.
type activeSession struct {
	info   *sshConnInfo
	cancel context.CancelFunc
}

func New(tsnetServer *tsnet.Server, platformInterface adapter.PlatformInterface, options *option.TailscaleSSHServerOptions, logger logger.ContextLogger) (*Server, error) {
	s := &Server{
		tsnetServer:       tsnetServer,
		platformInterface: platformInterface,
		logger:            logger,
		disablePTY:        options.DisablePTY,
		disableSFTP:       options.DisableSFTP,
		disableForwarding: options.DisableForwarding,
		done:              make(chan struct{}),
		activeConns:       make(map[*activeSession]struct{}),
	}
	s.serverCtx, s.serverCancel = context.WithCancel(context.Background())
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
			keyData, err := os.ReadFile(systemKey)
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
	keyData, err := os.ReadFile(keyPath)
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
	err = os.MkdirAll(dir, 0o700)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(keyPath, pemData, 0o600)
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
	close(s.done)
	s.serverCancel()
	s.access.Lock()
	for active := range s.activeConns {
		active.cancel()
	}
	s.access.Unlock()
	var err error
	if s.server != nil {
		err = s.server.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.sessionWg.Wait()
	if s.backend != nil {
		s.backend.Close()
	}
	return err
}

func (s *Server) serverConfig(ctx gliderssh.Context) *gossh.ServerConfig {
	config := &gossh.ServerConfig{
		NoClientAuthCallback: func(conn gossh.ConnMetadata) (*gossh.Permissions, error) {
			return s.authenticate(ctx, conn)
		},
		PasswordCallback: func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			return s.authenticate(ctx, conn)
		},
		PublicKeyCallback: func(conn gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			return s.authenticate(ctx, conn)
		},
		BannerCallback: func(conn gossh.ConnMetadata) string {
			connInfo := s.connInfoFromContext(ctx)
			if connInfo != nil && connInfo.action.Message != "" {
				return connInfo.action.Message
			}
			return ""
		},
	}
	return config
}

func (s *Server) authenticate(ctx gliderssh.Context, conn gossh.ConnMetadata) (*gossh.Permissions, error) {
	if s.connInfoFromContext(ctx) != nil {
		return &gossh.Permissions{}, nil
	}
	remoteAddrPort := M.AddrPortFromNet(conn.RemoteAddr())
	localBackend := s.tsnetServer.ExportLocalBackend()
	node, userProfile, found := localBackend.WhoIs("tcp", remoteAddrPort)
	// Every denial returns an empty *gossh.PartialSuccessError so x/crypto/ssh
	// stops offering further auth methods instead of re-running policy
	// evaluation (and hold-and-delegate) once per method.
	if !found {
		s.logger.Warn("SSH auth: unknown peer ", remoteAddrPort)
		return nil, &gossh.PartialSuccessError{}
	}
	netMap := localBackend.NetMap()
	if netMap == nil || netMap.SSHPolicy == nil {
		s.logger.Warn("SSH auth: no SSH policy")
		return nil, &gossh.PartialSuccessError{}
	}
	srcIP := remoteAddrPort.Addr()
	connInfo, err := s.evaluatePolicy(netMap.SSHPolicy, conn.User(), node, userProfile, srcIP)
	if err != nil {
		s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.User(), ": ", err)
		return nil, &gossh.PartialSuccessError{}
	}
	if connInfo.action.Reject {
		s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.User())
		return nil, &gossh.PartialSuccessError{}
	}
	connInfo.action0 = connInfo.action
	for hops := 0; connInfo.action.HoldAndDelegate != ""; hops++ {
		if hops >= 10 {
			s.logger.Info("SSH auth rejected: hold-and-delegate chain too long")
			return nil, &gossh.PartialSuccessError{}
		}
		delegatedAction, delegateErr := s.holdAndDelegate(ctx, connInfo.action, node, conn.User(), connInfo.localUser, srcIP)
		if delegateErr != nil {
			s.logger.Info("SSH auth rejected for ", userProfile.LoginName, ": ", delegateErr)
			return nil, &gossh.PartialSuccessError{}
		}
		connInfo.action = delegatedAction
		if connInfo.action.Reject {
			s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.User())
			return nil, &gossh.PartialSuccessError{}
		}
	}
	if !connInfo.action.Accept {
		s.logger.Info("SSH auth rejected for ", userProfile.LoginName, " -> ", conn.User())
		return nil, &gossh.PartialSuccessError{}
	}
	connInfo.sshUser = conn.User()
	connInfo.srcIP = srcIP
	connInfo.connID = newConnID()
	ctx.SetValue(sshConnContextKey{}, connInfo)
	s.logger.Info("SSH auth accepted: ", userProfile.LoginName, " -> ", connInfo.localUser)
	return &gossh.Permissions{}, nil
}

func (s *Server) evaluatePolicy(policy *tailcfg.SSHPolicy, sshUser string, node tailcfg.NodeView, userProfile tailcfg.UserProfile, srcIP netip.Addr) (*sshConnInfo, error) {
	now := time.Now()
	for _, rule := range policy.Rules {
		if rule.RuleExpires != nil && now.After(*rule.RuleExpires) {
			continue
		}
		if !s.matchPrincipals(rule.Principals, node, userProfile, srcIP) {
			continue
		}
		if rule.Action == nil {
			continue
		}
		if rule.Action.Reject {
			return &sshConnInfo{
				node:        node,
				userProfile: userProfile,
				action:      rule.Action,
			}, nil
		}
		localUser := s.matchSSHUser(rule.SSHUsers, sshUser)
		if localUser == "" {
			continue
		}
		return &sshConnInfo{
			node:        node,
			userProfile: userProfile,
			localUser:   localUser,
			action:      rule.Action,
			acceptEnv:   rule.AcceptEnv,
		}, nil
	}
	return nil, E.New("no matching SSH rule")
}

func (s *Server) matchPrincipals(principals []*tailcfg.SSHPrincipal, node tailcfg.NodeView, userProfile tailcfg.UserProfile, srcIP netip.Addr) bool {
	for _, p := range principals {
		if p == nil {
			continue
		}
		if p.Any {
			return true
		}
		if p.Node != "" && p.Node == node.StableID() {
			return true
		}
		if p.NodeIP != "" {
			principalIP, err := netip.ParseAddr(p.NodeIP)
			if err == nil && principalIP == srcIP {
				return true
			}
		}
		if p.UserLogin != "" && p.UserLogin == userProfile.LoginName {
			return true
		}
	}
	return false
}

func (s *Server) matchSSHUser(sshUsers map[string]string, requestedUser string) string {
	localUser, ok := sshUsers[requestedUser]
	if !ok {
		localUser, ok = sshUsers["*"]
		if !ok {
			return ""
		}
	}
	if localUser == "" {
		return ""
	}
	if localUser == "=" {
		return requestedUser
	}
	return localUser
}

func (s *Server) holdAndDelegate(ctx context.Context, action *tailcfg.SSHAction, node tailcfg.NodeView, sshUser string, localUser string, srcIP netip.Addr) (*tailcfg.SSHAction, error) {
	lb := s.tsnetServer.ExportLocalBackend()
	delegateURL := action.HoldAndDelegate
	addr4, addr6 := s.tsnetServer.TailscaleIPs()
	dstNodeIP := addr4
	if !dstNodeIP.IsValid() {
		dstNodeIP = addr6
	}
	srcNodeIP := srcIP
	if !srcNodeIP.IsValid() && node.Addresses().Len() > 0 {
		srcNodeIP = node.Addresses().At(0).Addr()
	}
	var dstNodeID string
	netMap := lb.NetMap()
	if netMap != nil && netMap.SelfNode.Valid() {
		dstNodeID = fmt.Sprint(int64(netMap.SelfNode.ID()))
	}
	// Escape interpolated values; $SSH_USER and $LOCAL_USER are client-controlled
	// (matchSSHUser "=" passes the requested name through). Numeric node IDs need
	// no escaping.
	replacer := strings.NewReplacer(
		"$SRC_NODE_IP", url.QueryEscape(srcNodeIP.String()),
		"$SRC_NODE_ID", fmt.Sprint(int64(node.ID())),
		"$DST_NODE_IP", url.QueryEscape(dstNodeIP.String()),
		"$DST_NODE_ID", dstNodeID,
		"$SSH_USER", url.QueryEscape(sshUser),
		"$LOCAL_USER", url.QueryEscape(localUser),
	)
	delegateURL = replacer.Replace(delegateURL)
	deadline := time.After(30 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.done:
			return nil, E.New("server closing")
		case <-deadline:
			return nil, E.New("hold and delegate timed out")
		default:
		}
		req, err := http.NewRequestWithContext(ctx, "GET", delegateURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := lb.DoNoiseRequest(req)
		if err != nil {
			backoffErr := s.delegateBackoff(ctx)
			if backoffErr != nil {
				return nil, backoffErr
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			s.logger.Warn("hold and delegate: unexpected status ", resp.Status)
			backoffErr := s.delegateBackoff(ctx)
			if backoffErr != nil {
				return nil, backoffErr
			}
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			backoffErr := s.delegateBackoff(ctx)
			if backoffErr != nil {
				return nil, backoffErr
			}
			continue
		}
		var newAction tailcfg.SSHAction
		err = json.Unmarshal(body, &newAction)
		if err != nil {
			backoffErr := s.delegateBackoff(ctx)
			if backoffErr != nil {
				return nil, backoffErr
			}
			continue
		}
		return &newAction, nil
	}
}

// delegateBackoff waits up to a second between hold-and-delegate retries,
// returning a non-nil error (so the caller never returns a nil action) when the
// connection or the server is shutting down.
func (s *Server) delegateBackoff(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return E.New("server closing")
	case <-time.After(time.Second):
		return nil
	}
}

func (s *Server) connInfoFromContext(ctx gliderssh.Context) *sshConnInfo {
	val := ctx.Value(sshConnContextKey{})
	if val == nil {
		return nil
	}
	return val.(*sshConnInfo)
}

func (s *Server) resolveConnUser(connInfo *sshConnInfo) (*adapter.PlatformUser, error) {
	connInfo.localUserOnce.Do(func() {
		connInfo.localUserInfo, connInfo.localUserErr = resolveLocalUser(s.platformInterface, connInfo.localUser)
	})
	return connInfo.localUserInfo, connInfo.localUserErr
}

func (s *Server) handleSession(session gliderssh.Session) {
	connInfo := s.connInfoFromContext(session.Context())
	s.sessionWg.Add(1)
	defer s.sessionWg.Done()
	ctx, cancel := context.WithCancel(session.Context())
	defer cancel()
	active := &activeSession{info: connInfo, cancel: cancel}
	s.access.Lock()
	s.activeConns[active] = struct{}{}
	s.access.Unlock()
	defer func() {
		s.access.Lock()
		delete(s.activeConns, active)
		s.access.Unlock()
	}()
	if connInfo.action.SessionDuration != 0 {
		timer := time.AfterFunc(connInfo.action.SessionDuration, func() {
			io.WriteString(session.Stderr(), "Session duration exceeded.\r\n")
			cancel()
		})
		defer timer.Stop()
	}
	subsystem := session.Subsystem()
	if subsystem == "sftp" {
		s.handleSFTP(ctx, session, connInfo)
		return
	}
	if subsystem != "" {
		fmt.Fprintf(session.Stderr(), "unsupported subsystem: %s\r\n", subsystem)
		session.Exit(1)
		return
	}
	localUser, err := s.resolveConnUser(connInfo)
	if err != nil {
		fmt.Fprintf(session.Stderr(), "failed to lookup user %s: %s\r\n", connInfo.localUser, err)
		session.Exit(1)
		return
	}
	err = verifyShellIdentity(localUser)
	if err != nil {
		s.logger.Warn("shell rejected for ", localUser.Username, ": ", err)
		fmt.Fprintf(session.Stderr(), "%s\r\n", err)
		session.Exit(1)
		return
	}
	var agentSocketPath string
	if connInfo.action.AllowAgentForwarding && !s.disableForwarding && gliderssh.AgentRequested(session) {
		agentListener, listenErr := gliderssh.NewAgentListener()
		if listenErr == nil {
			defer agentListener.Close()
			agentSocketPath = agentListener.Addr().String()
			// The agent socket is created as the server identity; hand it to the
			// target user so SSH_AUTH_SOCK is reachable after privileges drop.
			prepareErr := prepareAgentSocket(agentSocketPath, localUser.Uid, localUser.Gid)
			if prepareErr != nil {
				s.logger.Warn("prepare agent socket: ", prepareErr)
			}
			go gliderssh.ForwardAgentConnections(agentListener, session)
		}
	}
	env := s.buildEnvironment(session, connInfo, localUser)
	if agentSocketPath != "" {
		env = append(env, "SSH_AUTH_SOCK="+agentSocketPath)
	}
	ptyReq, winCh, isPty := session.Pty()
	session.DisablePTYEmulation()
	command := session.RawCommand()
	var term string
	var rows, cols uint16
	if isPty {
		term = ptyReq.Term
		rows = clampWindowDimension(ptyReq.Window.Height)
		cols = clampWindowDimension(ptyReq.Window.Width)
	}
	var rec *recording
	recorderList, onFailure := recorders(connInfo)
	if len(recorderList) > 0 {
		rec, err = s.startNewRecording(ctx, cancel, session, connInfo, localUser, recorderList, onFailure)
		if err != nil {
			var rejected *recordingRejectedError
			if errors.As(err, &rejected) && rejected.message != "" {
				io.WriteString(session.Stderr(), rejected.message+"\r\n")
			}
			s.logger.Error("recording: ", err)
			session.Exit(1)
			return
		}
		if rec != nil {
			defer rec.Close()
			// Cancel the session ctx before the recording is closed (defers run LIFO,
			// so this runs first), so the upload watcher observes the session as ended
			// on a clean final flush instead of misreading it as a mid-session upload
			// failure.
			defer cancel()
		}
	}
	shellSession, err := s.backend.OpenSession(shellRequest{
		User:    localUser,
		Command: command,
		Env:     env,
		Term:    term,
		Rows:    rows,
		Cols:    cols,
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
					shellSession.Resize(clampWindowDimension(win.Height), clampWindowDimension(win.Width))
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
		session.Exit(130)
	case exitStatus := <-exitCh:
		select {
		case <-outputDone:
		case <-ctx.Done():
		}
		session.Exit(int(exitStatus))
	}
}

func (s *Server) handleSFTP(ctx context.Context, session gliderssh.Session, connInfo *sshConnInfo) {
	if s.disableSFTP {
		fmt.Fprint(session.Stderr(), "SFTP is disabled.\r\n")
		session.Exit(1)
		return
	}
	localUser, err := s.resolveConnUser(connInfo)
	if err != nil {
		fmt.Fprintf(session.Stderr(), "failed to lookup user %s: %s\r\n", connInfo.localUser, err)
		session.Exit(1)
		return
	}
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
	err = verifyShellIdentity(localUser)
	if err != nil {
		s.logger.Warn("sftp rejected for ", localUser.Username, ": ", err)
		fmt.Fprintf(session.Stderr(), "%s\r\n", err)
		session.Exit(1)
		return
	}
	env := s.buildEnvironment(session, connInfo, localUser)
	sftpSession, err := s.backend.OpenSession(shellRequest{
		User:    localUser,
		Command: sftpCommand(sftpPath),
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

func (s *Server) buildEnvironment(session gliderssh.Session, connInfo *sshConnInfo, localUser *adapter.PlatformUser) []string {
	var env []string
	env = append(env,
		"USER="+localUser.Username,
		"HOME="+localUser.HomeDir,
		"SHELL="+localUser.Shell,
		"PATH="+defaultPathEnv(),
	)
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
	acceptEnv := connInfo.acceptEnv
	if len(acceptEnv) > 0 {
		netMap := s.tsnetServer.ExportLocalBackend().NetMap()
		if netMap == nil || !netMap.HasCap(tailcfg.NodeAttrSSHEnvironmentVariables) {
			acceptEnv = nil
		}
	}
	for _, clientEnv := range session.Environ() {
		name, _, found := strings.Cut(clientEnv, "=")
		if !found {
			continue
		}
		// TERM is already set authoritatively from the PTY request above; skip a
		// client-sent duplicate that would otherwise override it.
		if isPty && name == "TERM" {
			continue
		}
		if s.envAccepted(name, acceptEnv) {
			env = append(env, clientEnv)
		}
	}
	return env
}

func (s *Server) envAccepted(name string, extraPatterns []string) bool {
	// Never forward loader/shell-init variables, even if an AcceptEnv pattern
	// (e.g. "LD_*" or "*") would match: they allow code execution in a shell that
	// may run as another local user.
	if isDangerousEnv(name) {
		return false
	}
	// Never let a client override the variables the server sets authoritatively from
	// the resolved local user: a forwarded PATH/HOME/SHELL would otherwise win (execve
	// resolves duplicate keys last) and redirect command or identity resolution for the
	// spawned shell, even when an AcceptEnv pattern such as "*" matches.
	switch name {
	case "USER", "LOGNAME", "HOME", "SHELL", "PATH":
		return false
	}
	if name == "TERM" || name == "LANG" || strings.HasPrefix(name, "LC_") {
		return true
	}
	for _, pattern := range extraPatterns {
		matched, _ := path.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

func isDangerousEnv(name string) bool {
	if strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_") {
		return true
	}
	switch name {
	case "IFS", "ENV", "BASH_ENV", "SHELLOPTS", "BASHOPTS", "PS4", "GLOBIGNORE":
		return true
	}
	return false
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
	return s.connInfoFromContext(ctx).action.AllowLocalPortForwarding
}

func (s *Server) allowReverseForward(ctx gliderssh.Context, bindHost string, bindPort uint32) bool {
	if s.disableForwarding {
		return false
	}
	return s.connInfoFromContext(ctx).action.AllowRemotePortForwarding
}

func (s *Server) allowLocalUnixForward(ctx gliderssh.Context, socketPath string) (net.Conn, error) {
	if s.disableForwarding {
		return nil, gliderssh.ErrRejected
	}
	connInfo := s.connInfoFromContext(ctx)
	if !connInfo.action.AllowLocalPortForwarding {
		return nil, gliderssh.ErrRejected
	}
	localUser, err := s.resolveConnUser(connInfo)
	if err != nil {
		return nil, gliderssh.ErrRejected
	}
	opts := gliderssh.UnixForwardingOptions{
		AllowedDirectories: userSocketDirectories(localUser),
	}
	return gliderssh.NewLocalUnixForwardingCallback(opts)(ctx, socketPath)
}

func (s *Server) allowReverseUnixForward(ctx gliderssh.Context, socketPath string) (net.Listener, error) {
	if s.disableForwarding {
		return nil, gliderssh.ErrRejected
	}
	connInfo := s.connInfoFromContext(ctx)
	if !connInfo.action.AllowRemotePortForwarding {
		return nil, gliderssh.ErrRejected
	}
	localUser, err := s.resolveConnUser(connInfo)
	if err != nil {
		return nil, gliderssh.ErrRejected
	}
	opts := gliderssh.UnixForwardingOptions{
		AllowedDirectories: userSocketDirectories(localUser),
		BindUnlink:         true,
	}
	return gliderssh.NewReverseUnixForwardingCallback(opts)(ctx, socketPath)
}

func (s *Server) OnReconfig(cfg *wgcfg.Config, routerCfg *router.Config, dnsCfg *tsDNS.Config) {
	localBackend := s.tsnetServer.ExportLocalBackend()
	netMap := localBackend.NetMap()
	if netMap == nil || netMap.SSHPolicy == nil {
		return
	}
	s.access.Lock()
	connsToCheck := make([]*activeSession, 0, len(s.activeConns))
	for active := range s.activeConns {
		connsToCheck = append(connsToCheck, active)
	}
	s.access.Unlock()
	for _, active := range connsToCheck {
		connInfo := active.info
		newConnInfo, err := s.evaluatePolicy(netMap.SSHPolicy, connInfo.sshUser, connInfo.node, connInfo.userProfile, connInfo.srcIP)
		// A HoldAndDelegate rule re-evaluates to an action with Accept=false, so a
		// session granted via delegation must not be revoked just because Accept is
		// not set on the raw rule.
		if err == nil && !newConnInfo.action.Reject && (newConnInfo.action.Accept || newConnInfo.action.HoldAndDelegate != "") && newConnInfo.localUser == connInfo.localUser {
			continue
		}
		s.logger.Info("revoking SSH access for ", connInfo.userProfile.LoginName)
		active.cancel()
	}
}

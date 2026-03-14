package ocm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.OCMServiceOptions](registry, C.TypeOCM, NewService)
}

type errorResponse struct {
	Error errorDetails `json:"error"`
}

type errorDetails struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, message string) {
	writeJSONErrorWithCode(w, r, statusCode, errorType, "", message)
}

func writeJSONErrorWithCode(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, errorCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(errorResponse{
		Error: errorDetails{
			Type:    errorType,
			Code:    errorCode,
			Message: message,
		},
	})
}

func writePlainTextError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	_, _ = io.WriteString(w, message)
}

const (
	retryableUsageMessage = "current credential reached its usage limit; retry the request to use another credential"
	retryableUsageCode    = "credential_usage_exhausted"
)

func hasAlternativeCredential(provider credentialProvider, currentCredential credential, selection credentialSelection) bool {
	if provider == nil || currentCredential == nil {
		return false
	}
	for _, cred := range provider.allCredentials() {
		if cred == currentCredential {
			continue
		}
		if !selection.allows(cred) {
			continue
		}
		if cred.isUsable() {
			return true
		}
	}
	return false
}

func unavailableCredentialMessage(provider credentialProvider, fallback string) string {
	if provider == nil {
		return fallback
	}
	message := allRateLimitedError(provider.allCredentials()).Error()
	if message == "all credentials unavailable" && fallback != "" {
		return fallback
	}
	return message
}

func writeRetryableUsageError(w http.ResponseWriter, r *http.Request) {
	writeJSONErrorWithCode(w, r, http.StatusServiceUnavailable, "server_error", retryableUsageCode, retryableUsageMessage)
}

func writeNonRetryableCredentialError(w http.ResponseWriter, message string) {
	writePlainTextError(w, http.StatusBadRequest, message)
}

func writeCredentialUnavailableError(
	w http.ResponseWriter,
	r *http.Request,
	provider credentialProvider,
	currentCredential credential,
	selection credentialSelection,
	fallback string,
) {
	if hasAlternativeCredential(provider, currentCredential, selection) {
		writeRetryableUsageError(w, r)
		return
	}
	writeNonRetryableCredentialError(w, unavailableCredentialMessage(provider, fallback))
}

func credentialSelectionForUser(userConfig *option.OCMUser) credentialSelection {
	selection := credentialSelection{scope: credentialSelectionScopeAll}
	if userConfig != nil && !userConfig.AllowExternalUsage {
		selection.scope = credentialSelectionScopeNonExternal
		selection.filter = func(cred credential) bool {
			return !cred.isExternal()
		}
	}
	return selection
}

func isHopByHopHeader(header string) bool {
	switch strings.ToLower(header) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade", "host":
		return true
	default:
		return false
	}
}

func isReverseProxyHeader(header string) bool {
	lowerHeader := strings.ToLower(header)
	if strings.HasPrefix(lowerHeader, "cf-") {
		return true
	}
	switch lowerHeader {
	case "cdn-loop", "true-client-ip", "x-forwarded-for", "x-forwarded-proto", "x-real-ip":
		return true
	default:
		return false
	}
}

type Service struct {
	boxService.Adapter
	ctx             context.Context
	logger          log.ContextLogger
	options         option.OCMServiceOptions
	httpHeaders     http.Header
	listener        *listener.Listener
	tlsConfig       tls.ServerConfig
	httpServer      *http.Server
	userManager     *UserManager
	webSocketAccess sync.Mutex
	webSocketGroup  sync.WaitGroup
	webSocketConns  map[*webSocketSession]struct{}
	shuttingDown    bool

	// Legacy mode
	legacyCredential *defaultCredential
	legacyProvider   credentialProvider

	// Multi-credential mode
	providers      map[string]credentialProvider
	allCredentials []credential
	userConfigMap  map[string]*option.OCMUser
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.OCMServiceOptions) (adapter.Service, error) {
	err := validateOCMOptions(options)
	if err != nil {
		return nil, E.Cause(err, "validate options")
	}

	userManager := &UserManager{
		tokenMap: make(map[string]string),
	}

	service := &Service{
		Adapter:     boxService.NewAdapter(C.TypeOCM, tag),
		ctx:         ctx,
		logger:      logger,
		options:     options,
		httpHeaders: options.Headers.Build(),
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		userManager:    userManager,
		webSocketConns: make(map[*webSocketSession]struct{}),
	}

	if len(options.Credentials) > 0 {
		providers, allCredentials, err := buildOCMCredentialProviders(ctx, options, logger)
		if err != nil {
			return nil, E.Cause(err, "build credential providers")
		}
		service.providers = providers
		service.allCredentials = allCredentials

		userConfigMap := make(map[string]*option.OCMUser)
		for i := range options.Users {
			userConfigMap[options.Users[i].Name] = &options.Users[i]
		}
		service.userConfigMap = userConfigMap
	} else {
		cred, err := newDefaultCredential(ctx, "default", option.OCMDefaultCredentialOptions{
			CredentialPath: options.CredentialPath,
			UsagesPath:     options.UsagesPath,
			Detour:         options.Detour,
		}, logger)
		if err != nil {
			return nil, err
		}
		service.legacyCredential = cred
		service.legacyProvider = &singleCredentialProvider{cred: cred}
		service.allCredentials = []credential{cred}
	}

	if options.TLS != nil {
		tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
		if err != nil {
			return nil, err
		}
		service.tlsConfig = tlsConfig
	}

	return service, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}

	s.userManager.UpdateUsers(s.options.Users)

	for _, cred := range s.allCredentials {
		if extCred, ok := cred.(*externalCredential); ok && extCred.reverse && extCred.connectorURL != nil {
			extCred.reverseService = s
		}
		err := cred.start()
		if err != nil {
			return err
		}
		tag := cred.tagName()
		cred.setOnBecameUnusable(func() {
			s.interruptWebSocketSessionsForCredential(tag)
		})
	}
	if len(s.options.Credentials) > 0 {
		err := validateOCMCompositeCredentialModes(s.options, s.providers)
		if err != nil {
			return E.Cause(err, "validate loaded credentials")
		}
	}

	router := chi.NewRouter()
	router.Mount("/", s)

	s.httpServer = &http.Server{Handler: h2c.NewHandler(router, &http2.Server{})}

	if s.tlsConfig != nil {
		err := s.tlsConfig.Start()
		if err != nil {
			return E.Cause(err, "create TLS config")
		}
	}

	tcpListener, err := s.listener.ListenTCP()
	if err != nil {
		return err
	}

	if s.tlsConfig != nil {
		if !common.Contains(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{"h2"}, s.tlsConfig.NextProtos()...))
		}
		tcpListener = aTLS.NewListener(tcpListener, s.tlsConfig)
	}

	go func() {
		serveErr := s.httpServer.Serve(tcpListener)
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("serve error: ", serveErr)
		}
	}()

	return nil
}

func (s *Service) InterfaceUpdated() {
	for _, cred := range s.allCredentials {
		extCred, ok := cred.(*externalCredential)
		if !ok {
			continue
		}
		if extCred.reverse && extCred.connectorURL != nil {
			extCred.reverseService = s
			extCred.resetReverseContext()
			go extCred.connectorLoop()
		}
	}
}

func (s *Service) Close() error {
	webSocketSessions := s.startWebSocketShutdown()

	err := common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)
	for _, session := range webSocketSessions {
		session.Close()
	}
	s.webSocketGroup.Wait()

	for _, cred := range s.allCredentials {
		cred.close()
	}

	return err
}

func (s *Service) registerWebSocketSession(session *webSocketSession) bool {
	s.webSocketAccess.Lock()
	defer s.webSocketAccess.Unlock()

	if s.shuttingDown {
		return false
	}

	s.webSocketConns[session] = struct{}{}
	s.webSocketGroup.Add(1)
	return true
}

func (s *Service) unregisterWebSocketSession(session *webSocketSession) {
	s.webSocketAccess.Lock()
	_, loaded := s.webSocketConns[session]
	if loaded {
		delete(s.webSocketConns, session)
	}
	s.webSocketAccess.Unlock()

	if loaded {
		s.webSocketGroup.Done()
	}
}

func (s *Service) isShuttingDown() bool {
	s.webSocketAccess.Lock()
	defer s.webSocketAccess.Unlock()
	return s.shuttingDown
}

func (s *Service) interruptWebSocketSessionsForCredential(tag string) {
	s.webSocketAccess.Lock()
	var toClose []*webSocketSession
	for session := range s.webSocketConns {
		if session.credentialTag == tag {
			toClose = append(toClose, session)
		}
	}
	s.webSocketAccess.Unlock()
	for _, session := range toClose {
		session.Close()
	}
}

func (s *Service) startWebSocketShutdown() []*webSocketSession {
	s.webSocketAccess.Lock()
	defer s.webSocketAccess.Unlock()

	s.shuttingDown = true

	webSocketSessions := make([]*webSocketSession, 0, len(s.webSocketConns))
	for session := range s.webSocketConns {
		webSocketSessions = append(webSocketSessions, session)
	}
	return webSocketSessions
}

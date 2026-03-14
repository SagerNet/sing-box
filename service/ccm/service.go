package ccm

import (
	"context"
	"encoding/json"
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

const retryableUsageMessage = "current credential reached its usage limit; retry the request to use another credential"

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.CCMServiceOptions](registry, C.TypeCCM, NewService)
}

type errorResponse struct {
	Type      string       `json:"type"`
	Error     errorDetails `json:"error"`
	RequestID string       `json:"request_id,omitempty"`
}

type errorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func writeJSONError(w http.ResponseWriter, r *http.Request, statusCode int, errorType string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse{
		Type: "error",
		Error: errorDetails{
			Type:    errorType,
			Message: message,
		},
		RequestID: r.Header.Get("Request-Id"),
	})
}

func hasAlternativeCredential(provider credentialProvider, currentCredential Credential, selection credentialSelection) bool {
	if provider == nil || currentCredential == nil {
		return false
	}
	for _, credential := range provider.allCredentials() {
		if credential == currentCredential {
			continue
		}
		if !selection.allows(credential) {
			continue
		}
		if credential.isUsable() {
			return true
		}
	}
	return false
}

func unavailableCredentialMessage(provider credentialProvider, fallback string) string {
	if provider == nil {
		return fallback
	}
	message := allCredentialsUnavailableError(provider.allCredentials()).Error()
	if message == "all credentials unavailable" && fallback != "" {
		return fallback
	}
	return message
}

func writeRetryableUsageError(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, r, http.StatusTooManyRequests, "rate_limit_error", retryableUsageMessage)
}

func writeNonRetryableCredentialError(w http.ResponseWriter, r *http.Request, message string) {
	writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", message)
}

func writeCredentialUnavailableError(
	w http.ResponseWriter,
	r *http.Request,
	provider credentialProvider,
	currentCredential Credential,
	selection credentialSelection,
	fallback string,
) {
	if hasAlternativeCredential(provider, currentCredential, selection) {
		writeRetryableUsageError(w, r)
		return
	}
	writeNonRetryableCredentialError(w, r, unavailableCredentialMessage(provider, fallback))
}

func credentialSelectionForUser(userConfig *option.CCMUser) credentialSelection {
	selection := credentialSelection{scope: credentialSelectionScopeAll}
	if userConfig != nil && !userConfig.AllowExternalUsage {
		selection.scope = credentialSelectionScopeNonExternal
		selection.filter = func(credential Credential) bool {
			return !credential.isExternal()
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

func isAPIKeyHeader(header string) bool {
	switch strings.ToLower(header) {
	case "x-api-key", "api-key":
		return true
	default:
		return false
	}
}

type Service struct {
	boxService.Adapter
	ctx           context.Context
	logger        log.ContextLogger
	options       option.CCMServiceOptions
	httpHeaders   http.Header
	listener      *listener.Listener
	tlsConfig     tls.ServerConfig
	httpServer    *http.Server
	userManager   *UserManager
	trackingGroup sync.WaitGroup
	shuttingDown  bool

	// Legacy mode (single credential)
	legacyCredential *defaultCredential
	legacyProvider   credentialProvider

	// Multi-credential mode
	providers      map[string]credentialProvider
	allCredentials []Credential
	userConfigMap  map[string]*option.CCMUser
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.CCMServiceOptions) (adapter.Service, error) {
	initCCMUserAgent(logger)

	err := validateCCMOptions(options)
	if err != nil {
		return nil, E.Cause(err, "validate options")
	}

	userManager := &UserManager{
		tokenMap: make(map[string]string),
	}

	service := &Service{
		Adapter:     boxService.NewAdapter(C.TypeCCM, tag),
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
		userManager: userManager,
	}

	if len(options.Credentials) > 0 {
		providers, allCredentials, err := buildCredentialProviders(ctx, options, logger)
		if err != nil {
			return nil, E.Cause(err, "build credential providers")
		}
		service.providers = providers
		service.allCredentials = allCredentials

		userConfigMap := make(map[string]*option.CCMUser)
		for i := range options.Users {
			userConfigMap[options.Users[i].Name] = &options.Users[i]
		}
		service.userConfigMap = userConfigMap
	} else {
		credential, err := newDefaultCredential(ctx, "default", option.CCMDefaultCredentialOptions{
			CredentialPath: options.CredentialPath,
			UsagesPath:     options.UsagesPath,
			Detour:         options.Detour,
		}, logger)
		if err != nil {
			return nil, err
		}
		service.legacyCredential = credential
		service.legacyProvider = &singleCredentialProvider{credential: credential}
		service.allCredentials = []Credential{credential}
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

	for _, credential := range s.allCredentials {
		if external, ok := credential.(*externalCredential); ok && external.reverse && external.connectorURL != nil {
			external.reverseService = s
		}
		err := credential.start()
		if err != nil {
			return err
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
		if serveErr != nil && !E.IsClosed(serveErr) {
			s.logger.Error("serve error: ", serveErr)
		}
	}()

	return nil
}

func (s *Service) InterfaceUpdated() {
	for _, credential := range s.allCredentials {
		external, ok := credential.(*externalCredential)
		if !ok {
			continue
		}
		if external.reverse && external.connectorURL != nil {
			external.reverseService = s
			external.resetReverseContext()
			go external.connectorLoop()
		}
	}
}

func (s *Service) Close() error {
	err := common.Close(
		common.PtrOrNil(s.httpServer),
		common.PtrOrNil(s.listener),
		s.tlsConfig,
	)

	for _, credential := range s.allCredentials {
		credential.close()
	}

	return err
}

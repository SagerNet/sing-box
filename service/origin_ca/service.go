package originca

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/certificate"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/ntp"

	"golang.org/x/net/idna"
)

const (
	cloudflareOriginCAEndpoint = "https://api.cloudflare.com/client/v4/certificates"
	defaultRequestedValidity   = option.CloudflareOriginCARequestValidity5475
	defaultRequestTimeout      = 30 * time.Second
	defaultRenewBefore         = 30 * 24 * time.Hour
	minimumRenewRetryDelay     = time.Minute
	maximumRenewRetryDelay     = time.Hour
	certificateFileName        = "certificate.pem"
	privateKeyFileName         = "private_key.pem"
)

func RegisterCertificateProvider(registry *certificate.Registry) {
	certificate.Register[option.CloudflareOriginCACertificateProviderOptions](registry, C.TypeCloudflareOriginCA, NewCertificateProvider)
}

var _ adapter.CertificateProviderService = (*Service)(nil)

type Service struct {
	certificate.Adapter
	logger            log.ContextLogger
	ctx               context.Context
	cancel            context.CancelFunc
	done              chan struct{}
	httpClient        *http.Client
	requestTimeout    time.Duration
	dataDirectory     string
	apiToken          string
	originCAKey       string
	hostnames         []string
	requestType       option.CloudflareOriginCARequestType
	requestedValidity option.CloudflareOriginCARequestValidity
	renewBefore       time.Duration

	access             sync.RWMutex
	currentCertificate *tls.Certificate
	currentLeaf        *x509.Certificate
}

func NewCertificateProvider(ctx context.Context, logger log.ContextLogger, tag string, options option.CloudflareOriginCACertificateProviderOptions) (adapter.CertificateProviderService, error) {
	hostnames, err := normalizeHostnames(options.Hostnames)
	if err != nil {
		return nil, err
	}
	if len(hostnames) == 0 {
		return nil, E.New("hostnames is empty")
	}
	apiToken := strings.TrimSpace(options.APIToken)
	originCAKey := strings.TrimSpace(options.OriginCAKey)
	switch {
	case apiToken == "" && originCAKey == "":
		return nil, E.New("api_token or origin_ca_key is required")
	case apiToken != "" && originCAKey != "":
		return nil, E.New("api_token and origin_ca_key are mutually exclusive")
	}
	requestType := options.RequestType
	if requestType == "" {
		requestType = option.CloudflareOriginCARequestTypeOriginRSA
	}
	requestedValidity := options.RequestedValidity
	if requestedValidity == 0 {
		requestedValidity = defaultRequestedValidity
	}
	if !requestedValidity.IsKnown() {
		return nil, E.New("unsupported Cloudflare Origin CA requested validity: ", requestedValidity)
	}
	requestTimeout := options.RequestTimeout.Build()
	if requestTimeout < 0 {
		return nil, E.New("invalid request_timeout: ", time.Duration(options.RequestTimeout))
	}
	if requestTimeout == 0 {
		requestTimeout = defaultRequestTimeout
	}
	renewBefore := options.RenewBefore.Build()
	if renewBefore < 0 {
		return nil, E.New("invalid renew_before: ", time.Duration(options.RenewBefore))
	}
	validityDuration := time.Duration(requestedValidity) * 24 * time.Hour
	if renewBefore > 0 && renewBefore >= validityDuration {
		return nil, E.New("renew_before must be shorter than requested_validity")
	}
	ctx, cancel := context.WithCancel(ctx)
	serviceDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        ctx,
		Options:        option.DialerOptions{},
		RemoteIsDomain: true,
	})
	if err != nil {
		cancel()
		return nil, E.Cause(err, "create Cloudflare Origin CA dialer")
	}
	return &Service{
		Adapter: certificate.NewAdapter(C.TypeCloudflareOriginCA, tag),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		httpClient: &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return serviceDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			TLSClientConfig: &tls.Config{
				RootCAs: adapter.RootPoolFromContext(ctx),
				Time:    ntp.TimeFuncFromContext(ctx),
			},
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: requestTimeout,
			ExpectContinueTimeout: 2 * time.Second,
			ForceAttemptHTTP2:     true,
		}, Timeout: requestTimeout},
		requestTimeout:    requestTimeout,
		dataDirectory:     options.DataDirectory,
		apiToken:          apiToken,
		originCAKey:       originCAKey,
		hostnames:         hostnames,
		requestType:       requestType,
		requestedValidity: requestedValidity,
		renewBefore:       renewBefore,
	}, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	cachedCertificate, cachedLeaf, err := s.loadCachedCertificate()
	if err != nil {
		s.logger.Warn(E.Cause(err, "load cached Cloudflare Origin CA certificate"))
	} else if cachedCertificate != nil {
		s.setCurrentCertificate(cachedCertificate, cachedLeaf)
	}
	if cachedCertificate == nil {
		err = s.issueAndStoreCertificate()
		if err != nil {
			return err
		}
	} else if s.shouldRenew(cachedLeaf, time.Now()) {
		err = s.issueAndStoreCertificate()
		if err != nil {
			s.logger.Warn(E.Cause(err, "renew cached Cloudflare Origin CA certificate"))
		}
	}
	s.done = make(chan struct{})
	go s.refreshLoop()
	return nil
}

func (s *Service) Close() error {
	s.cancel()
	if done := s.done; done != nil {
		<-done
	}
	if transport, loaded := s.httpClient.Transport.(*http.Transport); loaded {
		transport.CloseIdleConnections()
	}
	return nil
}

func (s *Service) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.access.RLock()
	certificate := s.currentCertificate
	s.access.RUnlock()
	if certificate == nil {
		return nil, E.New("Cloudflare Origin CA certificate is unavailable")
	}
	return certificate, nil
}

func (s *Service) refreshLoop() {
	defer close(s.done)
	var retryDelay time.Duration
	for {
		waitDuration := retryDelay
		if waitDuration == 0 {
			waitDuration = s.nextRefreshDelay()
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-s.ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
		err := s.issueAndStoreCertificate()
		if err != nil {
			s.logger.Error(E.Cause(err, "renew Cloudflare Origin CA certificate"))
			retryDelay = s.nextRetryDelay()
			continue
		}
		retryDelay = 0
	}
}

func (s *Service) nextRefreshDelay() time.Duration {
	s.access.RLock()
	leaf := s.currentLeaf
	s.access.RUnlock()
	if leaf == nil {
		return minimumRenewRetryDelay
	}
	refreshAt := leaf.NotAfter.Add(-s.effectiveRenewBefore(leaf))
	delay := time.Until(refreshAt)
	if delay < minimumRenewRetryDelay {
		return minimumRenewRetryDelay
	}
	return delay
}

func (s *Service) nextRetryDelay() time.Duration {
	s.access.RLock()
	leaf := s.currentLeaf
	s.access.RUnlock()
	if leaf == nil {
		return minimumRenewRetryDelay
	}
	remaining := time.Until(leaf.NotAfter)
	if remaining <= minimumRenewRetryDelay {
		return minimumRenewRetryDelay
	}
	if remaining < maximumRenewRetryDelay {
		return max(remaining/2, minimumRenewRetryDelay)
	}
	return maximumRenewRetryDelay
}

func (s *Service) shouldRenew(leaf *x509.Certificate, now time.Time) bool {
	return !now.Before(leaf.NotAfter.Add(-s.effectiveRenewBefore(leaf)))
}

func (s *Service) effectiveRenewBefore(leaf *x509.Certificate) time.Duration {
	if s.renewBefore > 0 {
		return s.renewBefore
	}
	lifetime := leaf.NotAfter.Sub(leaf.NotBefore)
	if lifetime <= 0 {
		return 0
	}
	return min(lifetime/3, defaultRenewBefore)
}

func (s *Service) issueAndStoreCertificate() error {
	requestContext := s.ctx
	if s.requestTimeout > 0 {
		var cancel context.CancelFunc
		requestContext, cancel = context.WithTimeout(s.ctx, s.requestTimeout)
		defer cancel()
	}
	certificatePEM, privateKeyPEM, tlsCertificate, leaf, err := s.requestCertificate(requestContext)
	if err != nil {
		return err
	}
	if s.dataDirectory != "" {
		err = writePEMFile(filepath.Join(s.dataDirectory, certificateFileName), certificatePEM)
		if err != nil {
			return E.Cause(err, "store Cloudflare Origin CA certificate")
		}
		err = writePEMFile(filepath.Join(s.dataDirectory, privateKeyFileName), privateKeyPEM)
		if err != nil {
			return E.Cause(err, "store Cloudflare Origin CA private key")
		}
	}
	s.setCurrentCertificate(tlsCertificate, leaf)
	s.logger.Info("updated Cloudflare Origin CA certificate, expires at ", leaf.NotAfter.Format(time.RFC3339))
	return nil
}

func (s *Service) requestCertificate(ctx context.Context) ([]byte, []byte, *tls.Certificate, *x509.Certificate, error) {
	privateKey, err := generatePrivateKey(s.requestType)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	privateKeyPEM, err := encodePrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "encode private key")
	}
	certificateRequestPEM, err := createCertificateRequest(privateKey, s.hostnames)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "create certificate request")
	}
	requestBody, err := json.Marshal(originCARequest{
		CSR:               string(certificateRequestPEM),
		Hostnames:         s.hostnames,
		RequestType:       string(s.requestType),
		RequestedValidity: uint16(s.requestedValidity),
	})
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "marshal request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudflareOriginCAEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "create request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "sing-box/"+C.Version)
	if s.apiToken != "" {
		request.Header.Set("Authorization", "Bearer "+s.apiToken)
	} else {
		request.Header.Set("X-Auth-User-Service-Key", s.originCAKey)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "request certificate from Cloudflare")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "read Cloudflare response")
	}
	var responseEnvelope originCAResponse
	err = json.Unmarshal(responseBody, &responseEnvelope)
	if err != nil && response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil, nil, nil, nil, E.Cause(err, "decode Cloudflare response")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, nil, nil, nil, buildOriginCAError(response.StatusCode, responseEnvelope.Errors, responseBody)
	}
	if !responseEnvelope.Success {
		return nil, nil, nil, nil, buildOriginCAError(response.StatusCode, responseEnvelope.Errors, responseBody)
	}
	if responseEnvelope.Result.Certificate == "" {
		return nil, nil, nil, nil, E.New("Cloudflare Origin CA response is missing certificate data")
	}
	certificatePEM := []byte(responseEnvelope.Result.Certificate)
	tlsCertificate, leaf, err := parseKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "parse issued certificate")
	}
	if !s.matchesCertificate(leaf) {
		return nil, nil, nil, nil, E.New("issued Cloudflare Origin CA certificate does not match requested hostnames or key type")
	}
	return certificatePEM, privateKeyPEM, tlsCertificate, leaf, nil
}

func (s *Service) loadCachedCertificate() (*tls.Certificate, *x509.Certificate, error) {
	if s.dataDirectory == "" {
		return nil, nil, nil
	}
	certificatePath := filepath.Join(s.dataDirectory, certificateFileName)
	privateKeyPath := filepath.Join(s.dataDirectory, privateKeyFileName)
	certificatePEM, err := os.ReadFile(certificatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, E.Cause(err, "read ", certificatePath)
	}
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, E.Cause(err, "read ", privateKeyPath)
	}
	tlsCertificate, leaf, err := parseKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, nil, E.Cause(err, "parse cached key pair")
	}
	if time.Now().After(leaf.NotAfter) {
		return nil, nil, nil
	}
	if !s.matchesCertificate(leaf) {
		return nil, nil, nil
	}
	return tlsCertificate, leaf, nil
}

func (s *Service) matchesCertificate(leaf *x509.Certificate) bool {
	if leaf == nil {
		return false
	}
	leafHostnames := leaf.DNSNames
	if len(leafHostnames) == 0 && leaf.Subject.CommonName != "" {
		leafHostnames = []string{leaf.Subject.CommonName}
	}
	normalizedLeafHostnames, err := normalizeHostnames(leafHostnames)
	if err != nil {
		return false
	}
	if !slices.Equal(normalizedLeafHostnames, s.hostnames) {
		return false
	}
	switch s.requestType {
	case option.CloudflareOriginCARequestTypeOriginRSA:
		return leaf.PublicKeyAlgorithm == x509.RSA
	case option.CloudflareOriginCARequestTypeOriginECC:
		return leaf.PublicKeyAlgorithm == x509.ECDSA
	default:
		return false
	}
}

func (s *Service) setCurrentCertificate(certificate *tls.Certificate, leaf *x509.Certificate) {
	s.access.Lock()
	s.currentCertificate = certificate
	s.currentLeaf = leaf
	s.access.Unlock()
}

func normalizeHostnames(hostnames []string) ([]string, error) {
	normalizedHostnames := make([]string, 0, len(hostnames))
	seen := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		normalizedHostname, err := normalizeHostname(hostname)
		if err != nil {
			return nil, err
		}
		if _, loaded := seen[normalizedHostname]; loaded {
			continue
		}
		seen[normalizedHostname] = struct{}{}
		normalizedHostnames = append(normalizedHostnames, normalizedHostname)
	}
	slices.Sort(normalizedHostnames)
	return normalizedHostnames, nil
}

func normalizeHostname(hostname string) (string, error) {
	hostname = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(hostname, ".")))
	if hostname == "" {
		return "", E.New("hostname is empty")
	}
	if net.ParseIP(hostname) != nil {
		return "", E.New("hostname cannot be an IP address: ", hostname)
	}
	if strings.Contains(hostname, "*") {
		if !strings.HasPrefix(hostname, "*.") || strings.Count(hostname, "*") != 1 {
			return "", E.New("invalid wildcard hostname: ", hostname)
		}
		suffix := strings.TrimPrefix(hostname, "*.")
		if strings.Count(suffix, ".") == 0 {
			return "", E.New("wildcard hostname must cover a multi-label domain: ", hostname)
		}
		asciiSuffix, err := idna.Lookup.ToASCII(suffix)
		if err != nil {
			return "", E.Cause(err, "normalize hostname ", hostname)
		}
		return "*." + asciiSuffix, nil
	}
	asciiHostname, err := idna.Lookup.ToASCII(hostname)
	if err != nil {
		return "", E.Cause(err, "normalize hostname ", hostname)
	}
	return asciiHostname, nil
}

func generatePrivateKey(requestType option.CloudflareOriginCARequestType) (crypto.Signer, error) {
	switch requestType {
	case option.CloudflareOriginCARequestTypeOriginRSA:
		return rsa.GenerateKey(rand.Reader, 2048)
	case option.CloudflareOriginCARequestTypeOriginECC:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	default:
		return nil, E.New("unsupported Cloudflare Origin CA request type: ", requestType)
	}
}

func encodePrivateKey(privateKey crypto.Signer) ([]byte, error) {
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	}), nil
}

func createCertificateRequest(privateKey crypto.Signer, hostnames []string) ([]byte, error) {
	certificateRequestDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: hostnames[0]},
		DNSNames: hostnames,
	}, privateKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: certificateRequestDER,
	}), nil
}

func parseKeyPair(certificatePEM []byte, privateKeyPEM []byte) (*tls.Certificate, *x509.Certificate, error) {
	keyPair, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, nil, err
	}
	if len(keyPair.Certificate) == 0 {
		return nil, nil, E.New("certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, nil, err
	}
	keyPair.Leaf = leaf
	return &keyPair, leaf, nil
}

func writePEMFile(path string, content []byte) error {
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	err = os.WriteFile(tempPath, content, 0o600)
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func buildOriginCAError(statusCode int, responseErrors []originCAResponseError, responseBody []byte) error {
	if len(responseErrors) > 0 {
		messageList := make([]string, 0, len(responseErrors))
		for _, responseError := range responseErrors {
			if responseError.Message == "" {
				continue
			}
			if responseError.Code != 0 {
				messageList = append(messageList, responseError.Message+" (code "+strconv.Itoa(responseError.Code)+")")
			} else {
				messageList = append(messageList, responseError.Message)
			}
		}
		if len(messageList) > 0 {
			return E.New("Cloudflare Origin CA request failed: HTTP ", statusCode, " ", strings.Join(messageList, ", "))
		}
	}
	responseText := strings.TrimSpace(string(responseBody))
	if responseText == "" {
		return E.New("Cloudflare Origin CA request failed: HTTP ", statusCode)
	}
	return E.New("Cloudflare Origin CA request failed: HTTP ", statusCode, " ", responseText)
}

type originCARequest struct {
	CSR               string   `json:"csr"`
	Hostnames         []string `json:"hostnames"`
	RequestType       string   `json:"request_type"`
	RequestedValidity uint16   `json:"requested_validity"`
}

type originCAResponse struct {
	Success bool                    `json:"success"`
	Errors  []originCAResponseError `json:"errors"`
	Result  originCAResponseResult  `json:"result"`
}

type originCAResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type originCAResponseResult struct {
	Certificate string `json:"certificate"`
}

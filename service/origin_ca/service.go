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
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/certificate"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/ntp"
	"github.com/sagernet/sing/service"

	"github.com/caddyserver/certmagic"
)

const (
	cloudflareOriginCAEndpoint = "https://api.cloudflare.com/client/v4/certificates"
	defaultRequestedValidity   = option.CloudflareOriginCARequestValidity5475
	// min of 30 days and certmagic's 1/3 lifetime ratio (maintain.go)
	defaultRenewBefore = 30 * 24 * time.Hour
	// from certmagic retry backoff range (async.go)
	minimumRenewRetryDelay = time.Minute
	maximumRenewRetryDelay = time.Hour
	storageLockPrefix      = "cloudflare-origin-ca"
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
	timeFunc          func() time.Time
	httpClient        *http.Client
	storage           certmagic.Storage
	storageIssuerKey  string
	storageNamesKey   string
	storageLockKey    string
	apiToken          string
	originCAKey       string
	domain            []string
	requestType       option.CloudflareOriginCARequestType
	requestedValidity option.CloudflareOriginCARequestValidity

	access             sync.RWMutex
	currentCertificate *tls.Certificate
	currentLeaf        *x509.Certificate
}

func NewCertificateProvider(ctx context.Context, logger log.ContextLogger, tag string, options option.CloudflareOriginCACertificateProviderOptions) (adapter.CertificateProviderService, error) {
	domain, err := normalizeHostnames(options.Domain)
	if err != nil {
		return nil, err
	}
	if len(domain) == 0 {
		return nil, E.New("missing domain")
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
	ctx, cancel := context.WithCancel(ctx)
	httpClient, err := originCAHTTPClient(ctx, logger, options)
	if err != nil {
		cancel()
		return nil, err
	}
	var storage certmagic.Storage
	if options.DataDirectory != "" {
		storage = &certmagic.FileStorage{Path: options.DataDirectory}
	} else {
		storage = certmagic.Default.Storage
	}
	timeFunc := ntp.TimeFuncFromContext(ctx)
	if timeFunc == nil {
		timeFunc = time.Now
	}
	storageIssuerKey := C.TypeCloudflareOriginCA + "-" + string(requestType)
	storageNamesKey := (&certmagic.CertificateResource{SANs: slices.Clone(domain)}).NamesKey()
	storageLockKey := strings.Join([]string{
		storageLockPrefix,
		certmagic.StorageKeys.Safe(storageIssuerKey),
		certmagic.StorageKeys.Safe(storageNamesKey),
	}, "/")
	return &Service{
		Adapter:           certificate.NewAdapter(C.TypeCloudflareOriginCA, tag),
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
		timeFunc:          timeFunc,
		httpClient:        httpClient,
		storage:           storage,
		storageIssuerKey:  storageIssuerKey,
		storageNamesKey:   storageNamesKey,
		storageLockKey:    storageLockKey,
		apiToken:          apiToken,
		originCAKey:       originCAKey,
		domain:            domain,
		requestType:       requestType,
		requestedValidity: requestedValidity,
	}, nil
}

func originCAHTTPClient(ctx context.Context, logger log.ContextLogger, options option.CloudflareOriginCACertificateProviderOptions) (*http.Client, error) {
	httpClientOptions := common.PtrValueOrDefault(options.HTTPClient)
	httpClientManager := service.FromContext[adapter.HTTPClientManager](ctx)
	transport, err := httpClientManager.ResolveTransport(ctx, logger, httpClientOptions)
	if err != nil {
		return nil, E.Cause(err, "create Cloudflare Origin CA http client")
	}
	return &http.Client{Transport: transport}, nil
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
	} else if s.shouldRenew(cachedLeaf, s.timeFunc()) {
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
			s.access.RLock()
			leaf := s.currentLeaf
			s.access.RUnlock()
			if leaf == nil {
				waitDuration = minimumRenewRetryDelay
			} else {
				refreshAt := leaf.NotAfter.Add(-s.effectiveRenewBefore(leaf))
				waitDuration = refreshAt.Sub(s.timeFunc())
				if waitDuration < minimumRenewRetryDelay {
					waitDuration = minimumRenewRetryDelay
				}
			}
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
			s.access.RLock()
			leaf := s.currentLeaf
			s.access.RUnlock()
			if leaf == nil {
				retryDelay = minimumRenewRetryDelay
			} else {
				remaining := leaf.NotAfter.Sub(s.timeFunc())
				switch {
				case remaining <= minimumRenewRetryDelay:
					retryDelay = minimumRenewRetryDelay
				case remaining < maximumRenewRetryDelay:
					retryDelay = max(remaining/2, minimumRenewRetryDelay)
				default:
					retryDelay = maximumRenewRetryDelay
				}
			}
			continue
		}
		retryDelay = 0
	}
}

func (s *Service) shouldRenew(leaf *x509.Certificate, now time.Time) bool {
	return !now.Before(leaf.NotAfter.Add(-s.effectiveRenewBefore(leaf)))
}

func (s *Service) effectiveRenewBefore(leaf *x509.Certificate) time.Duration {
	lifetime := leaf.NotAfter.Sub(leaf.NotBefore)
	if lifetime <= 0 {
		return 0
	}
	return min(lifetime/3, defaultRenewBefore)
}

func (s *Service) issueAndStoreCertificate() error {
	err := s.storage.Lock(s.ctx, s.storageLockKey)
	if err != nil {
		return E.Cause(err, "lock Cloudflare Origin CA certificate storage")
	}
	defer func() {
		err = s.storage.Unlock(context.WithoutCancel(s.ctx), s.storageLockKey)
		if err != nil {
			s.logger.Warn(E.Cause(err, "unlock Cloudflare Origin CA certificate storage"))
		}
	}()
	cachedCertificate, cachedLeaf, err := s.loadCachedCertificate()
	if err != nil {
		s.logger.Warn(E.Cause(err, "load cached Cloudflare Origin CA certificate"))
	} else if cachedCertificate != nil && !s.shouldRenew(cachedLeaf, s.timeFunc()) {
		s.setCurrentCertificate(cachedCertificate, cachedLeaf)
		return nil
	}
	certificatePEM, privateKeyPEM, tlsCertificate, leaf, err := s.requestCertificate(s.ctx)
	if err != nil {
		return err
	}
	issuerData, err := json.Marshal(originCAIssuerData{
		RequestType:       s.requestType,
		RequestedValidity: s.requestedValidity,
	})
	if err != nil {
		return E.Cause(err, "encode Cloudflare Origin CA certificate metadata")
	}
	err = storeCertificateResource(s.ctx, s.storage, s.storageIssuerKey, certmagic.CertificateResource{
		SANs:           slices.Clone(s.domain),
		CertificatePEM: certificatePEM,
		PrivateKeyPEM:  privateKeyPEM,
		IssuerData:     issuerData,
	})
	if err != nil {
		return E.Cause(err, "store Cloudflare Origin CA certificate")
	}
	s.setCurrentCertificate(tlsCertificate, leaf)
	s.logger.Info("updated Cloudflare Origin CA certificate, expires at ", leaf.NotAfter.Format(time.RFC3339))
	return nil
}

func (s *Service) requestCertificate(ctx context.Context) ([]byte, []byte, *tls.Certificate, *x509.Certificate, error) {
	var privateKey crypto.Signer
	switch s.requestType {
	case option.CloudflareOriginCARequestTypeOriginRSA:
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		privateKey = rsaKey
	case option.CloudflareOriginCARequestTypeOriginECC:
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		privateKey = ecKey
	default:
		return nil, nil, nil, nil, E.New("unsupported Cloudflare Origin CA request type: ", s.requestType)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "encode private key")
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})
	certificateRequestDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: s.domain[0]},
		DNSNames: s.domain,
	}, privateKey)
	if err != nil {
		return nil, nil, nil, nil, E.Cause(err, "create certificate request")
	}
	certificateRequestPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: certificateRequestDER,
	})
	requestBody, err := json.Marshal(originCARequest{
		CSR:               string(certificateRequestPEM),
		Hostnames:         s.domain,
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
	defer s.httpClient.CloseIdleConnections()
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
	certificateResource, err := loadCertificateResource(s.ctx, s.storage, s.storageIssuerKey, s.storageNamesKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	tlsCertificate, leaf, err := parseKeyPair(certificateResource.CertificatePEM, certificateResource.PrivateKeyPEM)
	if err != nil {
		return nil, nil, E.Cause(err, "parse cached key pair")
	}
	if s.timeFunc().After(leaf.NotAfter) {
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
	if !slices.Equal(normalizedLeafHostnames, s.domain) {
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
		normalizedHostname := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(hostname, ".")))
		if normalizedHostname == "" {
			return nil, E.New("hostname is empty")
		}
		if net.ParseIP(normalizedHostname) != nil {
			return nil, E.New("hostname cannot be an IP address: ", normalizedHostname)
		}
		if strings.Contains(normalizedHostname, "*") {
			if !strings.HasPrefix(normalizedHostname, "*.") || strings.Count(normalizedHostname, "*") != 1 {
				return nil, E.New("invalid wildcard hostname: ", normalizedHostname)
			}
			suffix := strings.TrimPrefix(normalizedHostname, "*.")
			if strings.Count(suffix, ".") == 0 {
				return nil, E.New("wildcard hostname must cover a multi-label domain: ", normalizedHostname)
			}
			normalizedHostname = "*." + suffix
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

func storeCertificateResource(ctx context.Context, storage certmagic.Storage, issuerKey string, certificateResource certmagic.CertificateResource) error {
	metaBytes, err := json.MarshalIndent(certificateResource, "", "\t")
	if err != nil {
		return err
	}
	namesKey := certificateResource.NamesKey()
	keyValueList := []struct {
		key   string
		value []byte
	}{
		{
			key:   certmagic.StorageKeys.SitePrivateKey(issuerKey, namesKey),
			value: certificateResource.PrivateKeyPEM,
		},
		{
			key:   certmagic.StorageKeys.SiteCert(issuerKey, namesKey),
			value: certificateResource.CertificatePEM,
		},
		{
			key:   certmagic.StorageKeys.SiteMeta(issuerKey, namesKey),
			value: metaBytes,
		},
	}
	for i, item := range keyValueList {
		err = storage.Store(ctx, item.key, item.value)
		if err != nil {
			for j := i - 1; j >= 0; j-- {
				storage.Delete(ctx, keyValueList[j].key)
			}
			return err
		}
	}
	return nil
}

func loadCertificateResource(ctx context.Context, storage certmagic.Storage, issuerKey string, namesKey string) (certmagic.CertificateResource, error) {
	privateKeyPEM, err := storage.Load(ctx, certmagic.StorageKeys.SitePrivateKey(issuerKey, namesKey))
	if err != nil {
		return certmagic.CertificateResource{}, err
	}
	certificatePEM, err := storage.Load(ctx, certmagic.StorageKeys.SiteCert(issuerKey, namesKey))
	if err != nil {
		return certmagic.CertificateResource{}, err
	}
	metaBytes, err := storage.Load(ctx, certmagic.StorageKeys.SiteMeta(issuerKey, namesKey))
	if err != nil {
		return certmagic.CertificateResource{}, err
	}
	var certificateResource certmagic.CertificateResource
	err = json.Unmarshal(metaBytes, &certificateResource)
	if err != nil {
		return certmagic.CertificateResource{}, E.Cause(err, "decode Cloudflare Origin CA certificate metadata")
	}
	certificateResource.PrivateKeyPEM = privateKeyPEM
	certificateResource.CertificatePEM = certificatePEM
	return certificateResource, nil
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

type originCAIssuerData struct {
	RequestType       option.CloudflareOriginCARequestType     `json:"request_type,omitempty"`
	RequestedValidity option.CloudflareOriginCARequestValidity `json:"requested_validity,omitempty"`
}

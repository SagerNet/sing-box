package hysteria2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

const (
	sessionTTL             = time.Minute
	realmNamePattern       = `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`
	maxRequestBodyBytes    = 4 << 10
	maxAddresses           = 8
	nonceHexLength         = 32
	obfsHexLength          = 64
	eventChannelSize       = 16
	maxPendingAttempts     = 16
	connectResponseTimeout = 10 * time.Second
)

var realmPattern = regexp.MustCompile(realmNamePattern)

type contextKey int

const (
	contextKeyUser contextKey = iota
	contextKeySession
)

type realmUser struct {
	name      string
	maxRealms int
}

type realmSession struct {
	id        string
	realmID   string
	username  string
	addresses []string
	expires   time.Time
	events    chan realmEvent
	timer     *time.Timer
	done      chan struct{}
	closed    bool
	pending   map[string]chan punchResponsePayload
}

type realmEvent struct {
	kind string
	data any
}

type punchEvent struct {
	Addresses []string `json:"addresses"`
	Nonce     string   `json:"nonce"`
	Obfs      string   `json:"obfs"`
}

type punchResponsePayload struct {
	addresses []string
}

type server struct {
	access     sync.Mutex
	realms     map[string]*realmSession
	sessions   map[string]*realmSession
	userCounts map[string]int
	logger     log.ContextLogger
	tokenMap   map[string]*realmUser
}

func newServer(logger log.ContextLogger, tokenMap map[string]*realmUser) *server {
	return &server{
		realms:     make(map[string]*realmSession),
		sessions:   make(map[string]*realmSession),
		userCounts: make(map[string]int),
		logger:     logger,
		tokenMap:   tokenMap,
	}
}

func validateRealmID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if !realmPattern.MatchString(id) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, render.M{"error": "bad_request", "message": "invalid realm name"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) authBearer(name string, key contextKey, lookup func(r *http.Request, token string) (any, bool)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			bearer, token, found := strings.Cut(header, " ")
			if bearer != "Bearer" || !found {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, render.M{"error": "invalid_token", "message": "invalid " + name + " token"})
				return
			}
			value, authenticated := lookup(r, token)
			if !authenticated {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, render.M{"error": "invalid_token", "message": "invalid " + name + " token"})
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, value)))
		})
	}
}

func (s *server) authUser(next http.Handler) http.Handler {
	return s.authBearer("realm", contextKeyUser, func(_ *http.Request, token string) (any, bool) {
		user, authenticated := s.tokenMap[token]
		return user, authenticated
	})(next)
}

func (s *server) authSession(next http.Handler) http.Handler {
	return s.authBearer("session", contextKeySession, func(r *http.Request, token string) (any, bool) {
		sess := s.getSessionByToken(token)
		if sess == nil || sess.realmID != chi.URLParam(r, "id") {
			return nil, false
		}
		return sess, true
	})(next)
}

func (s *server) getSessionByToken(token string) *realmSession {
	s.access.Lock()
	defer s.access.Unlock()
	sess := s.sessions[token]
	if sess == nil || sess.closed || time.Now().After(sess.expires) {
		return nil
	}
	return sess
}

func (s *server) removeSessionLocked(sess *realmSession) {
	if sess.closed {
		return
	}
	sess.closed = true
	close(sess.done)
	if s.realms[sess.realmID] == sess {
		delete(s.realms, sess.realmID)
	}
	if _, found := s.sessions[sess.id]; found {
		s.userCounts[sess.username]--
		if s.userCounts[sess.username] <= 0 {
			delete(s.userCounts, sess.username)
		}
	}
	delete(s.sessions, sess.id)
	sess.timer.Stop()
	close(sess.events)
	for nonce, ch := range sess.pending {
		close(ch)
		delete(sess.pending, nonce)
	}
}

func (s *server) removeSession(sess *realmSession) {
	s.access.Lock()
	defer s.access.Unlock()
	s.removeSessionLocked(sess)
}

func (s *server) removeExpiredSession(sess *realmSession) bool {
	s.access.Lock()
	defer s.access.Unlock()
	if sess.closed || !time.Now().After(sess.expires) {
		return false
	}
	s.removeSessionLocked(sess)
	return true
}

func (s *server) closeAll() {
	s.access.Lock()
	defer s.access.Unlock()
	for _, sess := range s.sessions {
		s.removeSessionLocked(sess)
	}
}

func (s *server) registerPending(sess *realmSession, nonce string) (chan punchResponsePayload, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	if sess.closed || len(sess.pending) >= maxPendingAttempts {
		return nil, false
	}
	if _, exists := sess.pending[nonce]; exists {
		return nil, false
	}
	ch := make(chan punchResponsePayload, 1)
	sess.pending[nonce] = ch
	return ch, true
}

func (s *server) deliverPending(sess *realmSession, nonce string, payload punchResponsePayload) bool {
	s.access.Lock()
	defer s.access.Unlock()
	if sess.closed {
		return false
	}
	ch, found := sess.pending[nonce]
	if !found {
		return false
	}
	delete(sess.pending, nonce)
	select {
	case ch <- payload:
	default:
	}
	return true
}

func (s *server) cancelPending(sess *realmSession, nonce string) {
	s.access.Lock()
	defer s.access.Unlock()
	delete(sess.pending, nonce)
}

func (s *server) sendEvent(sess *realmSession, ev realmEvent) bool {
	s.access.Lock()
	defer s.access.Unlock()
	if sess.closed {
		return false
	}
	select {
	case sess.events <- ev:
		return true
	default:
		return false
	}
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(contextKeyUser).(*realmUser)
	id := chi.URLParam(r, "id")
	var req struct {
		Addresses []string `json:"addresses"`
	}
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "invalid json"})
		return
	}
	err = validateAddresses(req.Addresses)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	s.access.Lock()
	if _, exists := s.realms[id]; exists {
		s.access.Unlock()
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, render.M{"error": "realm_taken", "message": "realm already registered"})
		return
	}
	if user.maxRealms > 0 && s.userCounts[user.name] >= user.maxRealms {
		s.access.Unlock()
		render.Status(r, http.StatusTooManyRequests)
		render.JSON(w, r, render.M{"error": "realm_limit_reached", "message": "per-user realm limit reached"})
		return
	}
	var b [16]byte
	_, err = rand.Read(b[:])
	if err != nil {
		s.access.Unlock()
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, render.M{"error": "internal", "message": "entropy failure"})
		return
	}
	sess := &realmSession{
		id:        hex.EncodeToString(b[:]),
		realmID:   id,
		username:  user.name,
		addresses: append([]string(nil), req.Addresses...),
		expires:   time.Now().Add(sessionTTL),
		events:    make(chan realmEvent, eventChannelSize),
		done:      make(chan struct{}),
		pending:   make(map[string]chan punchResponsePayload),
	}
	s.realms[id] = sess
	s.sessions[sess.id] = sess
	s.userCounts[user.name]++
	sess.timer = time.AfterFunc(sessionTTL, func() {
		if s.removeExpiredSession(sess) {
			s.logger.Debug("[", sess.username, "] session expired realm=", sess.realmID)
		}
	})
	s.access.Unlock()
	s.logger.InfoContext(r.Context(), "[", user.name, "] registered realm=", id)
	render.JSON(w, r, render.M{
		"session_id": sess.id,
		"ttl":        int(sessionTTL.Seconds()),
	})
}

func (s *server) handleDeregister(w http.ResponseWriter, r *http.Request) {
	sess := r.Context().Value(contextKeySession).(*realmSession)
	s.logger.InfoContext(r.Context(), "[", sess.username, "] deregistered realm=", sess.realmID)
	s.removeSession(sess)
	render.NoContent(w, r)
}

func (s *server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	sess := r.Context().Value(contextKeySession).(*realmSession)
	var req struct {
		Addresses []string `json:"addresses"`
	}
	err := render.DecodeJSON(r.Body, &req)
	if err != nil && !errors.Is(err, io.EOF) {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "invalid json"})
		return
	}
	if req.Addresses != nil {
		err = validateAddresses(req.Addresses)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
			return
		}
	}
	s.access.Lock()
	sess.expires = time.Now().Add(sessionTTL)
	if req.Addresses != nil {
		sess.addresses = append([]string(nil), req.Addresses...)
	}
	sess.timer.Reset(sessionTTL)
	s.access.Unlock()
	s.logger.DebugContext(r.Context(), "[", sess.username, "] heartbeat realm=", sess.realmID)
	s.sendEvent(sess, realmEvent{kind: "heartbeat_ack", data: render.M{"ttl": int(sessionTTL.Seconds())}})
	render.JSON(w, r, render.M{"ttl": int(sessionTTL.Seconds())})
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sess := r.Context().Value(contextKeySession).(*realmSession)
	flusher, supportsFlusher := w.(http.Flusher)
	if !supportsFlusher {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, render.M{"error": "internal", "message": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-sess.events:
			if !open {
				return
			}
			data, _ := json.Marshal(ev.data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.kind, data)
			flusher.Flush()
		}
	}
}

func (s *server) handleConnect(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(contextKeyUser).(*realmUser)
	id := chi.URLParam(r, "id")
	var req struct {
		Addresses []string `json:"addresses"`
		Nonce     string   `json:"nonce"`
		Obfs      string   `json:"obfs"`
	}
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "invalid json"})
		return
	}
	err = validateAddresses(req.Addresses)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	err = validateHexField("nonce", req.Nonce, nonceHexLength)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	err = validateHexField("obfs", req.Obfs, obfsHexLength)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	s.access.Lock()
	// Any authenticated realm user may connect to a registered realm. The user name
	// is for logging and per-user registration quota, not an ownership boundary here.
	sess := s.realms[id]
	if sess == nil || sess.closed || time.Now().After(sess.expires) {
		s.access.Unlock()
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, render.M{"error": "realm_not_found", "message": "realm not registered"})
		return
	}
	serverAddresses := append([]string(nil), sess.addresses...)
	s.access.Unlock()

	respCh, ready := s.registerPending(sess, req.Nonce)
	if !ready {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, render.M{"error": "rate_limited", "message": "too many in-flight connect attempts"})
		return
	}
	defer s.cancelPending(sess, req.Nonce)

	if !s.sendEvent(sess, realmEvent{kind: "punch", data: punchEvent{Addresses: req.Addresses, Nonce: req.Nonce, Obfs: req.Obfs}}) {
		render.Status(r, http.StatusServiceUnavailable)
		render.JSON(w, r, render.M{"error": "rate_limited", "message": "server event buffer full"})
		return
	}
	s.logger.DebugContext(r.Context(), "[", user.name, "] connect realm=", id)

	timer := time.NewTimer(connectResponseTimeout)
	defer timer.Stop()
	select {
	case payload, open := <-respCh:
		if !open {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, render.M{"error": "realm_not_found", "message": "realm not registered"})
			return
		}
		if len(payload.addresses) > 0 {
			serverAddresses = payload.addresses
		}
	case <-timer.C:
	case <-sess.done:
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, render.M{"error": "realm_not_found", "message": "realm not registered"})
		return
	case <-r.Context().Done():
		return
	}
	render.JSON(w, r, render.M{
		"addresses": serverAddresses,
		"nonce":     req.Nonce,
		"obfs":      req.Obfs,
	})
}

func (s *server) handleConnectResponse(w http.ResponseWriter, r *http.Request) {
	sess := r.Context().Value(contextKeySession).(*realmSession)
	nonce := chi.URLParam(r, "nonce")
	err := validateHexField("nonce", nonce, nonceHexLength)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	var req struct {
		Addresses []string `json:"addresses"`
	}
	err = render.DecodeJSON(r.Body, &req)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": "invalid json"})
		return
	}
	err = validateAddresses(req.Addresses)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, render.M{"error": "bad_request", "message": err.Error()})
		return
	}
	delivered := s.deliverPending(sess, nonce, punchResponsePayload{addresses: append([]string(nil), req.Addresses...)})
	if !delivered {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, render.M{"error": "attempt_not_found", "message": "no pending attempt for nonce"})
		return
	}
	s.logger.DebugContext(r.Context(), "[", sess.username, "] connect-response realm=", sess.realmID)
	render.NoContent(w, r)
}

func validateAddresses(addresses []string) error {
	if len(addresses) == 0 {
		return E.New("at least one address required")
	}
	if len(addresses) > maxAddresses {
		return E.New("too many addresses (max ", maxAddresses, ")")
	}
	for _, address := range addresses {
		_, err := netip.ParseAddrPort(address)
		if err != nil {
			return E.New("invalid address: ", address)
		}
	}
	return nil
}

func validateHexField(name, value string, length int) error {
	if len(value) != length {
		return E.New(name, " must be ", length, " hex characters")
	}
	_, err := hex.DecodeString(value)
	if err != nil {
		return E.New(name, " must be valid hex")
	}
	return nil
}

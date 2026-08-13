package main

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errAuthInterrupted        = E.New("interrupted")
	errAuthChallengeWithdrawn = E.New("challenge no longer pending, waiting for the next one")
	errAuthDeadlineExpired    = E.New("challenge deadline expired; the server will retry the connection")
	errAuthNotInteractive     = E.New("authentication requires an interactive terminal")
)

var authInputIsTerminal = term.IsTerminal(int(os.Stdin.Fd())) && stderrIsTerminal

type vpnEndpointStatus interface {
	GetEndpointTag() string
}

func resolveVPNEndpoint[T vpnEndpointStatus](endpoints []T, endpointTag string, domain string) (T, error) {
	var zero T
	if endpointTag != "" {
		index := slices.IndexFunc(endpoints, func(it T) bool {
			return it.GetEndpointTag() == endpointTag
		})
		if index == -1 {
			return zero, E.New("endpoint not found: ", endpointTag)
		}
		return endpoints[index], nil
	}
	switch len(endpoints) {
	case 0:
		return zero, E.New("no ", domain, " endpoint is configured")
	case 1:
		return endpoints[0], nil
	default:
		return zero, E.New("multiple ", domain, " endpoints; select one with -e: ", strings.Join(common.Map(endpoints, func(it T) string {
			return it.GetEndpointTag()
		}), ", "))
	}
}

type vpnStatusWatcher[T any] struct {
	access    sync.Mutex
	updated   chan struct{}
	endpoints []T
	err       error
}

func newVPNStatusWatcher[T any](endpoints []T, recv func() ([]T, error)) *vpnStatusWatcher[T] {
	watcher := &vpnStatusWatcher[T]{
		updated:   make(chan struct{}),
		endpoints: endpoints,
	}
	go watcher.run(recv)
	return watcher
}

func (w *vpnStatusWatcher[T]) run(recv func() ([]T, error)) {
	for {
		endpoints, err := recv()
		w.access.Lock()
		if err != nil {
			w.err = err
		} else {
			w.endpoints = endpoints
		}
		close(w.updated)
		w.updated = make(chan struct{})
		w.access.Unlock()
		if err != nil {
			return
		}
	}
}

func (w *vpnStatusWatcher[T]) current() ([]T, <-chan struct{}, error) {
	w.access.Lock()
	defer w.access.Unlock()
	return w.endpoints, w.updated, w.err
}

type interactiveReadResult struct {
	line string
	err  error
}

type interactiveReadRequest struct {
	prompt string
	hidden bool
	result chan interactiveReadResult
}

type interactiveInput struct {
	requests chan interactiveReadRequest
}

func newInteractiveInput() *interactiveInput {
	input := &interactiveInput{requests: make(chan interactiveReadRequest)}
	go input.run()
	return input
}

func (i *interactiveInput) run() {
	for request := range i.requests {
		os.Stderr.WriteString(request.prompt)
		line, err := readTerminalLine(request.hidden)
		request.result <- interactiveReadResult{line: line, err: err}
	}
}

func readTerminalLine(hidden bool) (string, error) {
	if hidden {
		line, err := term.ReadPassword(int(os.Stdin.Fd()))
		os.Stderr.WriteString("\n")
		if err != nil {
			return "", err
		}
		return string(line), nil
	}
	var builder strings.Builder
	buffer := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buffer)
		if n > 0 {
			if buffer[0] == '\n' {
				break
			}
			builder.WriteByte(buffer[0])
		}
		if err != nil {
			if builder.Len() == 0 {
				return "", err
			}
			break
		}
	}
	return strings.TrimSuffix(builder.String(), "\r"), nil
}

type authPrompter struct {
	ctx      context.Context
	input    *interactiveInput
	once     sync.Once
	aborted  chan struct{}
	abortErr error
}

func (p *authPrompter) abort(cause error) {
	p.once.Do(func() {
		p.abortErr = cause
		close(p.aborted)
	})
}

func (p *authPrompter) read(prompt string, hidden bool) (string, error) {
	result := make(chan interactiveReadResult, 1)
	select {
	case p.input.requests <- interactiveReadRequest{prompt: prompt, hidden: hidden, result: result}:
	case <-p.aborted:
		return "", p.abortErr
	case <-p.ctx.Done():
		return "", errAuthInterrupted
	}
	select {
	case value := <-result:
		return value.line, value.err
	case <-p.aborted:
		return "", p.abortErr
	case <-p.ctx.Done():
		return "", errAuthInterrupted
	}
}

func (p *authPrompter) promptText(label string, value string) (string, error) {
	prompt := strings.TrimSuffix(label, ":")
	if value != "" {
		prompt += " [" + value + "]"
	}
	line, err := p.read(prompt+": ", false)
	if err != nil {
		return "", err
	}
	if line == "" {
		return value, nil
	}
	return line, nil
}

func (p *authPrompter) promptPassword(label string, value string) (string, error) {
	prompt := strings.TrimSuffix(label, ":")
	if value != "" {
		prompt += " (unchanged)"
	}
	line, err := p.read(prompt+": ", true)
	if err != nil {
		return "", err
	}
	if line == "" {
		return value, nil
	}
	return line, nil
}

func (p *authPrompter) promptConfirm(prompt string) (bool, error) {
	line, err := p.read(prompt, false)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func writeAuthLine(message string) {
	os.Stderr.WriteString(message + "\n")
}

func writeAuthError(domain string, message string) {
	os.Stderr.WriteString(domain + " auth: " + message + "\n")
}

func writeAuthHeader(endpointTag string, title string) {
	os.Stderr.WriteString("\n" + endpointTag + ": " + title + "\n")
}

func writeAuthBanner(banner string) {
	var output strings.Builder
	for line := range strings.SplitSeq(strings.ReplaceAll(banner, "\r\n", "\n"), "\n") {
		output.WriteString("  " + line + "\n")
	}
	os.Stderr.WriteString(output.String())
}

type submitOutcome int

const (
	submitRejected submitOutcome = iota
	submitStale
	submitFatal
)

func classifySubmitError(err error) (submitOutcome, string) {
	grpcStatus, isStatus := status.FromError(err)
	if !isStatus {
		return submitFatal, err.Error()
	}
	switch grpcStatus.Code() {
	case codes.Unavailable, codes.Canceled, codes.DeadlineExceeded, codes.Unauthenticated, codes.Unimplemented:
		return submitFatal, grpcStatus.Message()
	}
	if strings.Contains(grpcStatus.Message(), "no pending") {
		return submitStale, grpcStatus.Message()
	}
	return submitRejected, grpcStatus.Message()
}

func wrapAuthError(domain string, err error) error {
	if err == nil {
		return nil
	}
	_, isStatus := status.FromError(err)
	if isStatus {
		return err
	}
	return E.Cause(err, domain+" auth")
}

func formatVPNConnectedSince(connectedSince int64) string {
	if connectedSince == 0 {
		return ""
	}
	since := time.Unix(connectedSince, 0).Local()
	return since.Format(time.RFC3339) + " (" + time.Since(since).Truncate(time.Second).String() + ")"
}

func formatAuthDeadline(deadline int64) string {
	if deadline == 0 {
		return ""
	}
	return max(time.Until(time.Unix(deadline, 0)).Truncate(time.Second), 0).String()
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return address.IsLoopback()
}

func warnPlaintextAPIConnection() {
	parsed, err := url.Parse(commandAPIServerURL)
	if err != nil || parsed.Scheme == "https" || isLoopbackHost(parsed.Hostname()) {
		return
	}
	writeAuthLine("warning: submitting single sign-on credentials over a plaintext API connection")
}

func openURLInBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

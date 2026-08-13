package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

const (
	openConnectBrowserModeCallback = "callback"
	openConnectBrowserModeCookies  = "cookies"
	openConnectBrowserModeHeaders  = "headers"
)

const openConnectCallbackPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>single sign-on</title></head>
<body style="font-family:system-ui,sans-serif;text-align:center;margin-top:4em">
<h3>Single sign-on completed</h3>
<p>You may close this tab and return to the terminal.</p>
</body>
</html>
`

func deriveOpenConnectBrowserMode(request *daemon.OpenConnectBrowserRequest) (string, error) {
	callbackMode := len(request.GetCallbackURLPrefixes()) > 0
	cookieMode := request.GetFinalURL() != "" || len(request.GetCookieNames()) > 0 || len(request.GetEarlyCookieNames()) > 0
	headerMode := len(request.GetHeaderNames()) > 0
	selectedModes := common.Filter([]bool{callbackMode, cookieMode, headerMode}, func(it bool) bool {
		return it
	})
	invalidRequest := E.New("openconnect browser request must select exactly one completion mode")
	if len(selectedModes) != 1 {
		return "", invalidRequest
	}
	switch {
	case callbackMode:
		if len(common.Uniq(request.GetCallbackURLPrefixes())) != len(request.GetCallbackURLPrefixes()) {
			return "", invalidRequest
		}
		return openConnectBrowserModeCallback, nil
	case cookieMode:
		cookieNames := append(slices.Clone(request.GetCookieNames()), request.GetEarlyCookieNames()...)
		if len(common.Uniq(cookieNames)) != len(cookieNames) {
			return "", invalidRequest
		}
		return openConnectBrowserModeCookies, nil
	default:
		headerNames := common.Map(request.GetHeaderNames(), strings.ToLower)
		if len(common.Uniq(headerNames)) != len(headerNames) {
			return "", invalidRequest
		}
		return openConnectBrowserModeHeaders, nil
	}
}

func submitOpenConnectBrowser(
	ctx context.Context,
	client daemon.StartedServiceClient,
	prompter *authPrompter,
	endpointTag string,
	challenge *daemon.OpenConnectAuthChallenge,
	request *daemon.OpenConnectBrowserRequest,
) error {
	mode, err := deriveOpenConnectBrowserMode(request)
	if err != nil {
		return err
	}
	writeAuthHeader(endpointTag, "browser authentication")
	if challenge.GetError() != "" {
		writeAuthLine("previous attempt failed: " + challenge.GetError())
	}
	if challenge.GetMessage() != "" {
		writeAuthLine(challenge.GetMessage())
	}
	for {
		result, earlyFailure, collectErr := collectOpenConnectBrowserResult(ctx, prompter, mode, request)
		if collectErr != nil {
			return collectErr
		}
		warnPlaintextAPIConnection()
		_, submitErr := client.SubmitOpenConnectAuthResponse(ctx, &daemon.OpenConnectAuthResponseSubmission{
			EndpointTag: endpointTag,
			ChallengeID: challenge.GetId(),
			Response:    &daemon.OpenConnectAuthResponseSubmission_Browser{Browser: result},
		})
		if submitErr == nil {
			if earlyFailure {
				writeAuthLine("single sign-on failed; the client will retry authentication")
			}
			return nil
		}
		outcome, message := classifySubmitError(submitErr)
		switch outcome {
		case submitStale:
			return errAuthChallengeWithdrawn
		case submitFatal:
			return submitErr
		}
		writeAuthError("openconnect", "browser authentication rejected: "+message)
	}
}

func collectOpenConnectBrowserResult(
	ctx context.Context,
	prompter *authPrompter,
	mode string,
	request *daemon.OpenConnectBrowserRequest,
) (*daemon.OpenConnectBrowserResult, bool, error) {
	switch {
	case mode == openConnectBrowserModeCallback:
		target, err := parseOpenConnectCallbackTarget(request.GetCallbackURLPrefixes())
		if err != nil {
			return nil, false, err
		}
		if !authInputIsTerminal {
			return nil, false, errAuthNotInteractive
		}
		finalURL, err := runOpenConnectCallbackListener(ctx, prompter, target, request.GetUrl())
		if err != nil {
			return nil, false, err
		}
		return &daemon.OpenConnectBrowserResult{FinalURL: finalURL}, false, nil
	case mode == openConnectBrowserModeCookies && len(request.GetCookieNames()) > 0:
		if !authInputIsTerminal {
			return nil, false, errAuthNotInteractive
		}
		return promptOpenConnectBrowserCookies(prompter, request)
	case mode == openConnectBrowserModeHeaders:
		return nil, false, E.New("this single sign-on requires reading HTTP response headers, which cannot be done manually; use the sing-box desktop application")
	default:
		return nil, false, E.New("this single sign-on cannot be completed manually; use the sing-box desktop application")
	}
}

type openConnectCallbackTarget struct {
	scheme string
	host   string
	port   string
}

func (t openConnectCallbackTarget) resolve(requestURI string) string {
	return t.scheme + "://" + net.JoinHostPort(t.host, t.port) + requestURI
}

func parseOpenConnectCallbackTarget(prefixes []string) (openConnectCallbackTarget, error) {
	var target openConnectCallbackTarget
	for _, prefix := range prefixes {
		parsed, err := url.Parse(prefix)
		if err != nil || !isLoopbackHost(parsed.Hostname()) {
			return target, E.New("callback URL prefix is not on loopback: ", prefix)
		}
		if target.scheme == "" {
			target.scheme = parsed.Scheme
			target.host = parsed.Hostname()
		}
		if target.port == "" {
			target.port = parsed.Port()
		}
	}
	if target.port == "" {
		target.port = strconv.Itoa(int(commandAPIOpenConnectAuthFlagCallbackPort))
	}
	return target, nil
}

func runOpenConnectCallbackListener(ctx context.Context, prompter *authPrompter, target openConnectCallbackTarget, loginURL string) (string, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", target.port))
	if err != nil {
		return "", E.New("cannot listen on 127.0.0.1:", target.port, ": ", err.Error(), "; pass --callback-port")
	}
	defer listener.Close()
	writeAuthLine("Complete single sign-on in your browser; this command finishes automatically.")
	writeAuthLine("")
	writeAuthLine("  listening on  " + target.resolve("/"))
	writeAuthLine("  url           " + loginURL)
	writeAuthLine("")
	confirmed, err := prompter.promptConfirm("Open it now? [Y/n] ")
	if err != nil {
		return "", err
	}
	if !confirmed {
		writeAuthLine("waiting for the callback...")
	} else {
		openErr := openURLInBrowser(loginURL)
		if openErr != nil {
			writeAuthLine("failed to open the default browser: " + openErr.Error())
			writeAuthLine("waiting for the callback...")
		} else {
			writeAuthLine("opened in the default browser; waiting for the callback...")
		}
	}
	requestURIs := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case requestURIs <- request.RequestURI:
		default:
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Connection", "close")
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(openConnectCallbackPage))
	})}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()
	select {
	case requestURI := <-requestURIs:
		writeAuthLine("received callback")
		return target.resolve(requestURI), nil
	case <-prompter.aborted:
		return "", prompter.abortErr
	case <-ctx.Done():
		return "", errAuthInterrupted
	}
}

func promptOpenConnectBrowserCookies(prompter *authPrompter, request *daemon.OpenConnectBrowserRequest) (*daemon.OpenConnectBrowserResult, bool, error) {
	writeAuthLine("This single sign-on must be completed manually.")
	writeAuthLine("")
	step := 1
	writeAuthLine(" " + strconv.Itoa(step) + ". Open this URL in any browser:")
	writeAuthLine("      " + request.GetUrl())
	if request.GetFinalURL() != "" {
		step++
		writeAuthLine(" " + strconv.Itoa(step) + ". Sign in until the browser lands on:")
		writeAuthLine("      " + request.GetFinalURL())
	}
	step++
	writeAuthLine(" " + strconv.Itoa(step) + ". Open the developer tools (F12) > Application > Cookies, and read the")
	writeAuthLine("    value of the cookie listed below for that page.")
	writeAuthLine("")
	earlyCookieNames := request.GetEarlyCookieNames()
	var cookies []*daemon.OpenConnectBrowserCookie
	for index, name := range request.GetCookieNames() {
		prompt := `Cookie "` + name + `": `
		if index == 0 && len(earlyCookieNames) > 0 {
			prompt = `Cookie "` + name + `" (or "!" if the page reported an error): `
		}
		for {
			value, err := prompter.read(prompt, true)
			if err != nil {
				return nil, false, err
			}
			if index == 0 && len(earlyCookieNames) > 0 && value == "!" {
				earlyCookie, earlyErr := promptOpenConnectEarlyCookie(prompter, earlyCookieNames[0])
				if earlyErr != nil {
					return nil, false, earlyErr
				}
				return &daemon.OpenConnectBrowserResult{Cookies: []*daemon.OpenConnectBrowserCookie{earlyCookie}}, true, nil
			}
			if value == "" {
				writeAuthLine("cookie value must not be empty")
				continue
			}
			cookies = append(cookies, &daemon.OpenConnectBrowserCookie{Name: name, Value: value})
			break
		}
	}
	if len(cookies) == 1 {
		writeAuthLine("submitting 1 cookie")
	} else {
		writeAuthLine(F.ToString("submitting ", len(cookies), " cookies"))
	}
	return &daemon.OpenConnectBrowserResult{FinalURL: request.GetFinalURL(), Cookies: cookies}, false, nil
}

func promptOpenConnectEarlyCookie(prompter *authPrompter, name string) (*daemon.OpenConnectBrowserCookie, error) {
	for {
		value, err := prompter.read(`Error cookie "`+name+`": `, true)
		if err != nil {
			return nil, err
		}
		if value == "" {
			writeAuthLine("cookie value must not be empty")
			continue
		}
		return &daemon.OpenConnectBrowserCookie{Name: name, Value: value}, nil
	}
}

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var (
	commandAPIOpenConnectAuthFlagEndpoint     string
	commandAPIOpenConnectAuthFlagCallbackPort uint16
)

var commandAPIOpenConnectAuth = &cobra.Command{
	Use:   "auth",
	Short: "Answer OpenConnect authentication challenges",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runAPIOpenConnectAuth()
		if errors.Is(err, errAuthInterrupted) {
			writeAuthLine(`interrupted; the challenge is still pending — run "sing-box api openconnect auth" again, or "sing-box api openconnect cancel" to restart authentication`)
			os.Exit(130)
		}
		return wrapAuthError("openconnect", err)
	},
}

func init() {
	commandAPIOpenConnectAuth.Flags().StringVar(&commandAPIOpenConnectAuthFlagEndpoint, "endpoint", "", "OpenConnect endpoint tag (default: the only configured endpoint)")
	commandAPIOpenConnectAuth.Flags().Uint16Var(&commandAPIOpenConnectAuthFlagCallbackPort, "callback-port", 8020, "Local port for the browser single sign-on callback listener")
	commandAPIOpenConnect.AddCommand(commandAPIOpenConnectAuth)
}

func runAPIOpenConnectAuth() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stream, endpoints, err := subscribeOpenConnectStatus(ctx, client)
	if err != nil {
		return err
	}
	endpointStatus, err := resolveVPNEndpoint(endpoints, commandAPIOpenConnectAuthFlagEndpoint, "openconnect")
	if err != nil {
		return err
	}
	endpointTag := endpointStatus.GetEndpointTag()
	if endpointStatus.GetAuthChallenge() == nil {
		switch endpointStatus.GetState() {
		case adapter.OpenConnectStateConnected:
			return E.New("endpoint ", endpointTag, " is already connected")
		case adapter.OpenConnectStateError:
			return E.New("endpoint ", endpointTag, " failed: ", endpointStatus.GetError())
		}
	}
	watcher := newVPNStatusWatcher(endpoints, func() ([]*daemon.OpenConnectEndpointStatus, error) {
		return recvOpenConnectStatus(stream)
	})
	err = openConnectAuthLoop(ctx, client, watcher, newInteractiveInput(), endpointTag)
	if err != nil && ctx.Err() != nil {
		return errAuthInterrupted
	}
	return err
}

func openConnectAuthLoop(
	ctx context.Context,
	client daemon.StartedServiceClient,
	watcher *vpnStatusWatcher[*daemon.OpenConnectEndpointStatus],
	input *interactiveInput,
	endpointTag string,
) error {
	var (
		renderedID     string
		waitingPrinted bool
	)
	for {
		endpoints, updated, streamErr := watcher.current()
		if streamErr != nil {
			return streamErr
		}
		index := slices.IndexFunc(endpoints, func(it *daemon.OpenConnectEndpointStatus) bool {
			return it.GetEndpointTag() == endpointTag
		})
		if index == -1 {
			return E.New("endpoint not found: ", endpointTag)
		}
		endpointStatus := endpoints[index]
		challenge := endpointStatus.GetAuthChallenge()
		switch {
		case challenge == nil && endpointStatus.GetState() == adapter.OpenConnectStateConnected:
			os.Stdout.WriteString(endpointTag + ": connected\n")
			return nil
		case challenge == nil && endpointStatus.GetState() == adapter.OpenConnectStateError:
			return E.New("endpoint ", endpointTag, " failed: ", endpointStatus.GetError())
		case challenge != nil && challenge.GetId() != renderedID:
			renderedID = challenge.GetId()
			waitingPrinted = false
			handleErr := handleOpenConnectChallenge(ctx, client, watcher, input, endpointTag, challenge)
			switch {
			case handleErr == nil:
			case errors.Is(handleErr, errAuthChallengeWithdrawn):
				writeAuthLine(errAuthChallengeWithdrawn.Error())
			default:
				return handleErr
			}
			continue
		case challenge == nil && !waitingPrinted:
			waitingPrinted = true
			writeAuthLine("waiting for an authentication challenge on " + endpointTag + "...")
		}
		select {
		case <-updated:
		case <-ctx.Done():
			return errAuthInterrupted
		}
	}
}

func handleOpenConnectChallenge(
	ctx context.Context,
	client daemon.StartedServiceClient,
	watcher *vpnStatusWatcher[*daemon.OpenConnectEndpointStatus],
	input *interactiveInput,
	endpointTag string,
	challenge *daemon.OpenConnectAuthChallenge,
) error {
	prompter := &authPrompter{ctx: ctx, input: input, aborted: make(chan struct{})}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	go watchOpenConnectChallenge(watchCtx, watcher, endpointTag, challenge.GetId(), prompter)
	form := challenge.GetForm()
	browser := challenge.GetBrowser()
	switch {
	case form != nil:
		return submitOpenConnectForm(ctx, client, prompter, endpointTag, challenge, form)
	case browser != nil:
		return submitOpenConnectBrowser(ctx, client, prompter, endpointTag, challenge, browser)
	default:
		return E.New("unsupported authentication challenge")
	}
}

func watchOpenConnectChallenge(
	ctx context.Context,
	watcher *vpnStatusWatcher[*daemon.OpenConnectEndpointStatus],
	endpointTag string,
	challengeID string,
	prompter *authPrompter,
) {
	for {
		endpoints, updated, streamErr := watcher.current()
		if streamErr != nil {
			prompter.abort(streamErr)
			return
		}
		index := slices.IndexFunc(endpoints, func(it *daemon.OpenConnectEndpointStatus) bool {
			return it.GetEndpointTag() == endpointTag
		})
		if index == -1 || endpoints[index].GetAuthChallenge().GetId() != challengeID {
			prompter.abort(errAuthChallengeWithdrawn)
			return
		}
		select {
		case <-updated:
		case <-ctx.Done():
			return
		}
	}
}

func submitOpenConnectForm(
	ctx context.Context,
	client daemon.StartedServiceClient,
	prompter *authPrompter,
	endpointTag string,
	challenge *daemon.OpenConnectAuthChallenge,
	form *daemon.OpenConnectAuthForm,
) error {
	if !authInputIsTerminal {
		return errAuthNotInteractive
	}
	writeAuthHeader(endpointTag, "authentication")
	preambleWritten := false
	if challenge.GetBanner() != "" {
		writeAuthBanner(challenge.GetBanner())
		preambleWritten = true
	}
	if challenge.GetError() != "" {
		writeAuthLine("previous attempt failed: " + challenge.GetError())
		preambleWritten = true
	}
	if challenge.GetMessage() != "" {
		writeAuthLine(challenge.GetMessage())
		preambleWritten = true
	}
	if preambleWritten {
		writeAuthLine("")
	}
	for {
		values := make(map[string]string, len(form.GetFields()))
		for _, field := range form.GetFields() {
			value, err := promptOpenConnectField(prompter, field)
			if err != nil {
				return err
			}
			values[field.GetSubmissionKey()] = value
		}
		_, err := client.SubmitOpenConnectAuthResponse(ctx, &daemon.OpenConnectAuthResponseSubmission{
			EndpointTag: endpointTag,
			ChallengeID: challenge.GetId(),
			Response: &daemon.OpenConnectAuthResponseSubmission_Form{
				Form: &daemon.OpenConnectAuthFormResponse{Values: values},
			},
		})
		if err == nil {
			return nil
		}
		outcome, message := classifySubmitError(err)
		switch outcome {
		case submitStale:
			return errAuthChallengeWithdrawn
		case submitFatal:
			return err
		}
		writeAuthError("openconnect", "submit rejected: "+message)
	}
}

func promptOpenConnectField(prompter *authPrompter, field *daemon.OpenConnectAuthFormField) (string, error) {
	label := field.GetLabel()
	if label == "" {
		label = field.GetName()
	}
	switch field.GetKind() {
	case "text":
		return prompter.promptText(label, field.GetValue())
	case "password":
		return prompter.promptPassword(label, field.GetValue())
	case "select":
		return promptOpenConnectSelect(prompter, label, field.GetOptions(), field.GetValue())
	default:
		return "", E.New("unsupported authentication field kind: ", field.GetKind())
	}
}

func promptOpenConnectSelect(prompter *authPrompter, label string, options []*daemon.OpenConnectAuthFormChoice, defaultValue string) (string, error) {
	prompt := strings.TrimSuffix(label, ":")
	var menu strings.Builder
	menu.WriteString(prompt + ":\n")
	for index, option := range options {
		optionLabel := option.GetLabel()
		if optionLabel == "" {
			optionLabel = option.GetValue()
		}
		menu.WriteString("  " + strconv.Itoa(index+1) + ") " + optionLabel)
		if option.GetValue() == defaultValue {
			menu.WriteString("  [default]")
		}
		menu.WriteString("\n")
	}
	os.Stderr.WriteString(menu.String())
	for {
		line, err := prompter.read(prompt+": ", false)
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" && defaultValue != "" {
			return defaultValue, nil
		}
		selected, parseErr := strconv.Atoi(line)
		if parseErr == nil && selected >= 1 && selected <= len(options) {
			return options[selected-1].GetValue(), nil
		}
		if slices.ContainsFunc(options, func(it *daemon.OpenConnectAuthFormChoice) bool {
			return it.GetValue() == line
		}) {
			return line, nil
		}
		writeAuthLine("select a number between 1 and " + strconv.Itoa(len(options)))
	}
}

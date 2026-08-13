package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/spf13/cobra"
)

var commandAPIOpenVPNAuthFlagEndpoint string

var commandAPIOpenVPNAuth = &cobra.Command{
	Use:   "auth",
	Short: "Answer OpenVPN authentication challenges",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := runAPIOpenVPNAuth()
		if errors.Is(err, errAuthInterrupted) {
			writeAuthLine(`interrupted; the challenge is still pending — run "sing-box api openvpn auth" again, or "sing-box api openvpn cancel" to stop the client`)
			os.Exit(130)
		}
		return wrapAuthError("openvpn", err)
	},
}

func init() {
	commandAPIOpenVPNAuth.Flags().StringVar(&commandAPIOpenVPNAuthFlagEndpoint, "endpoint", "", "OpenVPN endpoint tag (default: the only configured endpoint)")
	commandAPIOpenVPN.AddCommand(commandAPIOpenVPNAuth)
}

func runAPIOpenVPNAuth() error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stream, endpoints, err := subscribeOpenVPNStatus(ctx, client)
	if err != nil {
		return err
	}
	endpointStatus, err := resolveVPNEndpoint(endpoints, commandAPIOpenVPNAuthFlagEndpoint, "openvpn")
	if err != nil {
		return err
	}
	endpointTag := endpointStatus.GetEndpointTag()
	if endpointStatus.GetChallenge() == nil {
		switch endpointStatus.GetState() {
		case adapter.OpenVPNStateConnected:
			return E.New("endpoint ", endpointTag, " is already connected")
		case adapter.OpenVPNStateError:
			return E.New("endpoint ", endpointTag, " failed: ", endpointStatus.GetError())
		}
	}
	watcher := newVPNStatusWatcher(endpoints, func() ([]*daemon.OpenVPNEndpointStatus, error) {
		return recvOpenVPNStatus(stream)
	})
	err = openVPNAuthLoop(ctx, client, watcher, newInteractiveInput(), endpointTag)
	if err != nil && ctx.Err() != nil {
		return errAuthInterrupted
	}
	return err
}

func openVPNAuthLoop(
	ctx context.Context,
	client daemon.StartedServiceClient,
	watcher *vpnStatusWatcher[*daemon.OpenVPNEndpointStatus],
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
		index := slices.IndexFunc(endpoints, func(it *daemon.OpenVPNEndpointStatus) bool {
			return it.GetEndpointTag() == endpointTag
		})
		if index == -1 {
			return E.New("endpoint not found: ", endpointTag)
		}
		endpointStatus := endpoints[index]
		challenge := endpointStatus.GetChallenge()
		switch {
		case challenge == nil && endpointStatus.GetState() == adapter.OpenVPNStateConnected:
			os.Stdout.WriteString(endpointTag + ": connected\n")
			return nil
		case challenge == nil && endpointStatus.GetState() == adapter.OpenVPNStateError:
			return E.New("endpoint ", endpointTag, " failed: ", endpointStatus.GetError())
		case challenge != nil && challenge.GetId() != renderedID:
			renderedID = challenge.GetId()
			waitingPrinted = false
			handleErr := handleOpenVPNChallenge(ctx, client, watcher, input, endpointTag, challenge)
			switch {
			case handleErr == nil:
			case errors.Is(handleErr, errAuthChallengeWithdrawn):
				writeAuthLine(errAuthChallengeWithdrawn.Error())
			case errors.Is(handleErr, errAuthDeadlineExpired):
				writeAuthError("openvpn", errAuthDeadlineExpired.Error())
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

func handleOpenVPNChallenge(
	ctx context.Context,
	client daemon.StartedServiceClient,
	watcher *vpnStatusWatcher[*daemon.OpenVPNEndpointStatus],
	input *interactiveInput,
	endpointTag string,
	challenge *daemon.OpenVPNChallenge,
) error {
	prompter := &authPrompter{ctx: ctx, input: input, aborted: make(chan struct{})}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	go watchOpenVPNChallenge(watchCtx, watcher, endpointTag, challenge.GetId(), prompter)
	switch challenge.GetKind() {
	case openVPNChallengeCredentials:
		return submitOpenVPNCredentials(ctx, client, prompter, endpointTag, challenge)
	case openVPNChallengeSecret:
		return submitOpenVPNSecret(ctx, client, prompter, endpointTag, challenge)
	case openVPNChallengeMessage:
		writeAuthHeader(endpointTag, "notice")
		writeAuthLine(challenge.GetMessage() + openVPNRemainingSuffix(challenge))
		return nil
	case openVPNChallengeOpenURL:
		return openOpenVPNChallengeURL(prompter, endpointTag, challenge)
	default:
		return E.New("unsupported authentication challenge kind: ", challenge.GetKind())
	}
}

func watchOpenVPNChallenge(
	ctx context.Context,
	watcher *vpnStatusWatcher[*daemon.OpenVPNEndpointStatus],
	endpointTag string,
	challengeID string,
	prompter *authPrompter,
) {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	for {
		endpoints, updated, streamErr := watcher.current()
		if streamErr != nil {
			prompter.abort(streamErr)
			return
		}
		index := slices.IndexFunc(endpoints, func(it *daemon.OpenVPNEndpointStatus) bool {
			return it.GetEndpointTag() == endpointTag
		})
		if index == -1 || endpoints[index].GetChallenge().GetId() != challengeID {
			prompter.abort(errAuthChallengeWithdrawn)
			return
		}
		var expired <-chan time.Time
		deadline := endpoints[index].GetChallenge().GetDeadline()
		if deadline != 0 {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(time.Until(time.Unix(deadline, 0)))
			expired = timer.C
		}
		select {
		case <-updated:
		case <-expired:
			prompter.abort(errAuthDeadlineExpired)
			return
		case <-ctx.Done():
			return
		}
	}
}

func submitOpenVPNCredentials(
	ctx context.Context,
	client daemon.StartedServiceClient,
	prompter *authPrompter,
	endpointTag string,
	challenge *daemon.OpenVPNChallenge,
) error {
	if !authInputIsTerminal {
		return errAuthNotInteractive
	}
	writeAuthHeader(endpointTag, "authentication")
	if challenge.GetPreviousError() != "" {
		writeAuthLine("previous attempt failed: " + challenge.GetPreviousError())
		writeAuthLine("")
	}
	secretLabel := challenge.GetSecretMessage()
	if secretLabel == "" {
		secretLabel = "Secret"
	}
	for {
		username, err := prompter.promptText("Username", challenge.GetUsername())
		if err != nil {
			return err
		}
		password, err := prompter.promptPassword("Password", "")
		if err != nil {
			return err
		}
		secret, err := prompter.read(strings.TrimSuffix(secretLabel, ":")+": ", !challenge.GetEcho())
		if err != nil {
			return err
		}
		answered, err := submitOpenVPNChallengeResponse(ctx, client, &daemon.OpenVPNChallengeSubmission{
			EndpointTag: endpointTag,
			ChallengeID: challenge.GetId(),
			Username:    username,
			Password:    password,
			Secret:      secret,
		})
		if err != nil {
			return err
		}
		if answered {
			return nil
		}
	}
}

func submitOpenVPNSecret(
	ctx context.Context,
	client daemon.StartedServiceClient,
	prompter *authPrompter,
	endpointTag string,
	challenge *daemon.OpenVPNChallenge,
) error {
	if !authInputIsTerminal {
		return errAuthNotInteractive
	}
	writeAuthHeader(endpointTag, "authentication")
	contextWritten := false
	if challenge.GetPreviousError() != "" {
		writeAuthLine("previous attempt failed: " + challenge.GetPreviousError())
		contextWritten = true
	}
	if challenge.GetUsername() != "" {
		writeAuthLine("user: " + challenge.GetUsername())
		contextWritten = true
	}
	if contextWritten {
		writeAuthLine("")
	}
	label := challenge.GetMessage()
	if challenge.GetDeadline() != 0 {
		if label != "" {
			writeAuthLine(label + openVPNRemainingSuffix(challenge))
		}
		label = "Code"
	}
	if label == "" {
		label = "Secret"
	}
	for {
		secret, err := prompter.read(strings.TrimSuffix(label, ":")+": ", !challenge.GetEcho())
		if err != nil {
			return err
		}
		answered, err := submitOpenVPNChallengeResponse(ctx, client, &daemon.OpenVPNChallengeSubmission{
			EndpointTag: endpointTag,
			ChallengeID: challenge.GetId(),
			Secret:      secret,
		})
		if err != nil {
			return err
		}
		if answered {
			return nil
		}
	}
}

func submitOpenVPNChallengeResponse(ctx context.Context, client daemon.StartedServiceClient, submission *daemon.OpenVPNChallengeSubmission) (bool, error) {
	_, err := client.SubmitOpenVPNChallengeResponse(ctx, submission)
	if err == nil {
		return true, nil
	}
	outcome, message := classifySubmitError(err)
	switch outcome {
	case submitStale:
		return false, errAuthChallengeWithdrawn
	case submitFatal:
		return false, err
	}
	writeAuthError("openvpn", "submit rejected: "+message)
	return false, nil
}

func openOpenVPNChallengeURL(prompter *authPrompter, endpointTag string, challenge *daemon.OpenVPNChallenge) error {
	writeAuthHeader(endpointTag, "authentication")
	if challenge.GetPreviousError() != "" {
		writeAuthLine("previous attempt failed: " + challenge.GetPreviousError())
	}
	writeAuthLine("Complete authentication in your browser:")
	writeAuthLine("")
	writeAuthLine("  " + challenge.GetUrl())
	writeAuthLine("")
	if authInputIsTerminal {
		confirmed, err := prompter.promptConfirm("Open it now? [Y/n] ")
		if err != nil {
			return err
		}
		if confirmed {
			openErr := openURLInBrowser(challenge.GetUrl())
			if openErr != nil {
				writeAuthLine("failed to open the default browser: " + openErr.Error())
			} else {
				writeAuthLine("opened in the default browser; waiting for the server" + openVPNRemainingSuffix(challenge))
				return nil
			}
		}
	}
	writeAuthLine("waiting for the server" + openVPNRemainingSuffix(challenge))
	return nil
}

func openVPNRemainingSuffix(challenge *daemon.OpenVPNChallenge) string {
	if challenge.GetDeadline() == 0 {
		return ""
	}
	return " (" + formatAuthDeadline(challenge.GetDeadline()) + " remaining)"
}

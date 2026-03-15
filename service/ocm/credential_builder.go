package ocm

import (
	"context"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func buildOCMCredentialProviders(
	ctx context.Context,
	options option.OCMServiceOptions,
	logger log.ContextLogger,
) (map[string]credentialProvider, []Credential, error) {
	allCredentialMap := make(map[string]Credential)
	var allCredentials []Credential
	providers := make(map[string]credentialProvider)

	// Pass 1: create default and external credentials
	for _, credentialOption := range options.Credentials {
		switch credentialOption.Type {
		case "default":
			credential, err := newDefaultCredential(ctx, credentialOption.Tag, credentialOption.DefaultOptions, logger)
			if err != nil {
				return nil, nil, err
			}
			allCredentialMap[credentialOption.Tag] = credential
			allCredentials = append(allCredentials, credential)
			providers[credentialOption.Tag] = &singleCredentialProvider{credential: credential}
		case "external":
			credential, err := newExternalCredential(ctx, credentialOption.Tag, credentialOption.ExternalOptions, logger)
			if err != nil {
				return nil, nil, err
			}
			allCredentialMap[credentialOption.Tag] = credential
			allCredentials = append(allCredentials, credential)
			providers[credentialOption.Tag] = &singleCredentialProvider{credential: credential}
		}
	}

	// Pass 2: create balancer providers
	for _, credentialOption := range options.Credentials {
		if credentialOption.Type == "balancer" {
			subCredentials, err := resolveCredentialTags(credentialOption.BalancerOptions.Credentials, allCredentialMap, credentialOption.Tag)
			if err != nil {
				return nil, nil, err
			}
			providers[credentialOption.Tag] = newBalancerProvider(subCredentials, credentialOption.BalancerOptions.Strategy, time.Duration(credentialOption.BalancerOptions.PollInterval), credentialOption.BalancerOptions.RebalanceThreshold, logger)
		}
	}

	return providers, allCredentials, nil
}

func resolveCredentialTags(tags []string, allCredentials map[string]Credential, parentTag string) ([]Credential, error) {
	credentials := make([]Credential, 0, len(tags))
	for _, tag := range tags {
		credential, exists := allCredentials[tag]
		if !exists {
			return nil, E.New("credential ", parentTag, " references unknown credential: ", tag)
		}
		credentials = append(credentials, credential)
	}
	if len(credentials) == 0 {
		return nil, E.New("credential ", parentTag, " has no sub-credentials")
	}
	return credentials, nil
}

func validateOCMOptions(options option.OCMServiceOptions) error {
	tags := make(map[string]bool)
	credentialTypes := make(map[string]string)
	for _, credential := range options.Credentials {
		if tags[credential.Tag] {
			return E.New("duplicate credential tag: ", credential.Tag)
		}
		tags[credential.Tag] = true
		credentialTypes[credential.Tag] = credential.Type
		if credential.Type == "default" || credential.Type == "" {
			if credential.DefaultOptions.Reserve5h > 99 {
				return E.New("credential ", credential.Tag, ": reserve_5h must be at most 99")
			}
			if credential.DefaultOptions.ReserveWeekly > 99 {
				return E.New("credential ", credential.Tag, ": reserve_weekly must be at most 99")
			}
			if credential.DefaultOptions.Limit5h > 100 {
				return E.New("credential ", credential.Tag, ": limit_5h must be at most 100")
			}
			if credential.DefaultOptions.LimitWeekly > 100 {
				return E.New("credential ", credential.Tag, ": limit_weekly must be at most 100")
			}
			if credential.DefaultOptions.Reserve5h > 0 && credential.DefaultOptions.Limit5h > 0 {
				return E.New("credential ", credential.Tag, ": reserve_5h and limit_5h are mutually exclusive")
			}
			if credential.DefaultOptions.ReserveWeekly > 0 && credential.DefaultOptions.LimitWeekly > 0 {
				return E.New("credential ", credential.Tag, ": reserve_weekly and limit_weekly are mutually exclusive")
			}
		}
		if credential.Type == "external" {
			if credential.ExternalOptions.Token == "" {
				return E.New("credential ", credential.Tag, ": external credential requires token")
			}
			if credential.ExternalOptions.Reverse && credential.ExternalOptions.URL == "" {
				return E.New("credential ", credential.Tag, ": reverse external credential requires url")
			}
		}
		if credential.Type == "balancer" {
			switch credential.BalancerOptions.Strategy {
			case "", C.BalancerStrategyLeastUsed, C.BalancerStrategyRoundRobin, C.BalancerStrategyRandom, C.BalancerStrategyFallback:
			default:
				return E.New("credential ", credential.Tag, ": unknown balancer strategy: ", credential.BalancerOptions.Strategy)
			}
			if credential.BalancerOptions.RebalanceThreshold < 0 {
				return E.New("credential ", credential.Tag, ": rebalance_threshold must not be negative")
			}
		}
	}

	singleCredential := len(options.Credentials) == 1
	for _, user := range options.Users {
		if user.Credential == "" && !singleCredential {
			return E.New("user ", user.Name, " must specify credential in multi-credential mode")
		}
		if user.Credential != "" && !tags[user.Credential] {
			return E.New("user ", user.Name, " references unknown credential: ", user.Credential)
		}
		if user.ExternalCredential != "" {
			if !tags[user.ExternalCredential] {
				return E.New("user ", user.Name, " references unknown external_credential: ", user.ExternalCredential)
			}
			if credentialTypes[user.ExternalCredential] != "external" {
				return E.New("user ", user.Name, ": external_credential must reference an external type credential")
			}
		}
	}

	return nil
}

func validateOCMCompositeCredentialModes(
	options option.OCMServiceOptions,
	providers map[string]credentialProvider,
) error {
	for _, credentialOption := range options.Credentials {
		if credentialOption.Type != "balancer" {
			continue
		}

		provider, exists := providers[credentialOption.Tag]
		if !exists {
			return E.New("unknown credential: ", credentialOption.Tag)
		}

		for _, subCred := range provider.allCredentials() {
			if !subCred.isAvailable() {
				continue
			}
			if subCred.ocmIsAPIKeyMode() {
				return E.New(
					"credential ", credentialOption.Tag,
					" references API key default credential ", subCred.tagName(),
					"; balancer and fallback only support OAuth default credentials",
				)
			}
		}
	}

	return nil
}

func credentialForUser(
	userConfigMap map[string]*option.OCMUser,
	providers map[string]credentialProvider,
	username string,
) (credentialProvider, error) {
	userConfig, exists := userConfigMap[username]
	if !exists {
		return nil, E.New("no credential mapping for user: ", username)
	}
	if userConfig.Credential == "" {
		for _, provider := range providers {
			return provider, nil
		}
		return nil, E.New("no credential available")
	}
	provider, exists := providers[userConfig.Credential]
	if !exists {
		return nil, E.New("unknown credential: ", userConfig.Credential)
	}
	return provider, nil
}


package ccm

import (
	"context"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func buildCredentialProviders(
	ctx context.Context,
	options option.CCMServiceOptions,
	logger log.ContextLogger,
) (map[string]credentialProvider, []credential, error) {
	allCredentialMap := make(map[string]credential)
	var allCreds []credential
	providers := make(map[string]credentialProvider)

	// Pass 1: create default and external credentials
	for _, credOpt := range options.Credentials {
		switch credOpt.Type {
		case "default":
			cred, err := newDefaultCredential(ctx, credOpt.Tag, credOpt.DefaultOptions, logger)
			if err != nil {
				return nil, nil, err
			}
			allCredentialMap[credOpt.Tag] = cred
			allCreds = append(allCreds, cred)
			providers[credOpt.Tag] = &singleCredentialProvider{cred: cred}
		case "external":
			cred, err := newExternalCredential(ctx, credOpt.Tag, credOpt.ExternalOptions, logger)
			if err != nil {
				return nil, nil, err
			}
			allCredentialMap[credOpt.Tag] = cred
			allCreds = append(allCreds, cred)
			providers[credOpt.Tag] = &singleCredentialProvider{cred: cred}
		}
	}

	// Pass 2: create balancer providers
	for _, credOpt := range options.Credentials {
		if credOpt.Type == "balancer" {
			subCredentials, err := resolveCredentialTags(credOpt.BalancerOptions.Credentials, allCredentialMap, credOpt.Tag)
			if err != nil {
				return nil, nil, err
			}
			providers[credOpt.Tag] = newBalancerProvider(subCredentials, credOpt.BalancerOptions.Strategy, time.Duration(credOpt.BalancerOptions.PollInterval), credOpt.BalancerOptions.RebalanceThreshold, logger)
		}
	}

	return providers, allCreds, nil
}

func resolveCredentialTags(tags []string, allCredentials map[string]credential, parentTag string) ([]credential, error) {
	credentials := make([]credential, 0, len(tags))
	for _, tag := range tags {
		cred, exists := allCredentials[tag]
		if !exists {
			return nil, E.New("credential ", parentTag, " references unknown credential: ", tag)
		}
		credentials = append(credentials, cred)
	}
	if len(credentials) == 0 {
		return nil, E.New("credential ", parentTag, " has no sub-credentials")
	}
	return credentials, nil
}

func validateCCMOptions(options option.CCMServiceOptions) error {
	hasCredentials := len(options.Credentials) > 0
	hasLegacyPath := options.CredentialPath != ""
	hasLegacyUsages := options.UsagesPath != ""
	hasLegacyDetour := options.Detour != ""

	if hasCredentials && hasLegacyPath {
		return E.New("credential_path and credentials are mutually exclusive")
	}
	if hasCredentials && hasLegacyUsages {
		return E.New("usages_path and credentials are mutually exclusive; use usages_path on individual credentials")
	}
	if hasCredentials && hasLegacyDetour {
		return E.New("detour and credentials are mutually exclusive; use detour on individual credentials")
	}

	if hasCredentials {
		tags := make(map[string]bool)
		credentialTypes := make(map[string]string)
		for _, cred := range options.Credentials {
			if tags[cred.Tag] {
				return E.New("duplicate credential tag: ", cred.Tag)
			}
			tags[cred.Tag] = true
			credentialTypes[cred.Tag] = cred.Type
			if cred.Type == "default" || cred.Type == "" {
				if cred.DefaultOptions.Reserve5h > 99 {
					return E.New("credential ", cred.Tag, ": reserve_5h must be at most 99")
				}
				if cred.DefaultOptions.ReserveWeekly > 99 {
					return E.New("credential ", cred.Tag, ": reserve_weekly must be at most 99")
				}
				if cred.DefaultOptions.Limit5h > 100 {
					return E.New("credential ", cred.Tag, ": limit_5h must be at most 100")
				}
				if cred.DefaultOptions.LimitWeekly > 100 {
					return E.New("credential ", cred.Tag, ": limit_weekly must be at most 100")
				}
				if cred.DefaultOptions.Reserve5h > 0 && cred.DefaultOptions.Limit5h > 0 {
					return E.New("credential ", cred.Tag, ": reserve_5h and limit_5h are mutually exclusive")
				}
				if cred.DefaultOptions.ReserveWeekly > 0 && cred.DefaultOptions.LimitWeekly > 0 {
					return E.New("credential ", cred.Tag, ": reserve_weekly and limit_weekly are mutually exclusive")
				}
			}
			if cred.Type == "external" {
				if cred.ExternalOptions.Token == "" {
					return E.New("credential ", cred.Tag, ": external credential requires token")
				}
				if cred.ExternalOptions.Reverse && cred.ExternalOptions.URL == "" {
					return E.New("credential ", cred.Tag, ": reverse external credential requires url")
				}
			}
			if cred.Type == "balancer" {
				switch cred.BalancerOptions.Strategy {
				case "", C.BalancerStrategyLeastUsed, C.BalancerStrategyRoundRobin, C.BalancerStrategyRandom, C.BalancerStrategyFallback:
				default:
					return E.New("credential ", cred.Tag, ": unknown balancer strategy: ", cred.BalancerOptions.Strategy)
				}
				if cred.BalancerOptions.RebalanceThreshold < 0 {
					return E.New("credential ", cred.Tag, ": rebalance_threshold must not be negative")
				}
			}
		}

		for _, user := range options.Users {
			if user.Credential == "" {
				return E.New("user ", user.Name, " must specify credential in multi-credential mode")
			}
			if !tags[user.Credential] {
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
	}

	return nil
}

func credentialForUser(
	userConfigMap map[string]*option.CCMUser,
	providers map[string]credentialProvider,
	legacyProvider credentialProvider,
	username string,
) (credentialProvider, error) {
	if legacyProvider != nil {
		return legacyProvider, nil
	}
	userConfig, exists := userConfigMap[username]
	if !exists {
		return nil, E.New("no credential mapping for user: ", username)
	}
	provider, exists := providers[userConfig.Credential]
	if !exists {
		return nil, E.New("unknown credential: ", userConfig.Credential)
	}
	return provider, nil
}

func noUserCredentialProvider(
	providers map[string]credentialProvider,
	legacyProvider credentialProvider,
	options option.CCMServiceOptions,
) credentialProvider {
	if legacyProvider != nil {
		return legacyProvider
	}
	if len(options.Credentials) > 0 {
		tag := options.Credentials[0].Tag
		return providers[tag]
	}
	return nil
}

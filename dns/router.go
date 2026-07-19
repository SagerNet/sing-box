package dns

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var (
	_ adapter.DNSRouter                 = (*Router)(nil)
	_ adapter.DNSRuleSetUpdateValidator = (*Router)(nil)
)

type Router struct {
	ctx                   context.Context
	logger                logger.ContextLogger
	transport             adapter.DNSTransportManager
	outbound              adapter.OutboundManager
	client                adapter.DNSClient
	rawRules              []option.DNSRule
	rules                 []adapter.DNSRule
	defaultDomainStrategy C.DomainStrategy
	dnsReverseMapping     *freelru.Cache[netip.Addr, string]
	platformInterface     adapter.PlatformInterface
	legacyDNSMode         bool
	rulesAccess           sync.RWMutex
	started               bool
	closing               bool
}

func NewRouter(ctx context.Context, logFactory log.Factory, options option.DNSOptions) (*Router, error) {
	router := &Router{
		ctx:                   ctx,
		logger:                logFactory.NewLogger("dns"),
		transport:             service.FromContext[adapter.DNSTransportManager](ctx),
		outbound:              service.FromContext[adapter.OutboundManager](ctx),
		rawRules:              make([]option.DNSRule, 0, len(options.Rules)),
		rules:                 make([]adapter.DNSRule, 0, len(options.Rules)),
		defaultDomainStrategy: C.DomainStrategy(options.Strategy),
	}
	if options.DNSClientOptions.IndependentCache {
		deprecated.Report(ctx, deprecated.OptionIndependentDNSCache)
	}
	var optimisticTimeout time.Duration
	optimisticOptions := common.PtrValueOrDefault(options.DNSClientOptions.Optimistic)
	if optimisticOptions.Enabled {
		if options.DNSClientOptions.DisableCache {
			return nil, E.New("`optimistic` is conflict with `disable_cache`")
		}
		if options.DNSClientOptions.DisableExpire {
			return nil, E.New("`optimistic` is conflict with `disable_expire`")
		}
		optimisticTimeout = time.Duration(optimisticOptions.Timeout)
		if optimisticTimeout == 0 {
			optimisticTimeout = 3 * 24 * time.Hour
		}
	}
	router.client = NewClient(ClientOptions{
		Context:           ctx,
		Timeout:           time.Duration(options.DNSClientOptions.Timeout),
		DisableCache:      options.DNSClientOptions.DisableCache,
		DisableExpire:     options.DNSClientOptions.DisableExpire,
		OptimisticTimeout: optimisticTimeout,
		CacheCapacity:     options.DNSClientOptions.CacheCapacity,
		ClientSubnet:      options.DNSClientOptions.ClientSubnet.Build(netip.Prefix{}),
		RDRC: func() adapter.RDRCStore {
			cacheFile := service.FromContext[adapter.CacheFile](ctx)
			if cacheFile == nil {
				return nil
			}
			if !cacheFile.StoreRDRC() {
				return nil
			}
			return cacheFile
		},
		DNSCache: func() adapter.DNSCacheStore {
			cacheFile := service.FromContext[adapter.CacheFile](ctx)
			if cacheFile == nil {
				return nil
			}
			if !cacheFile.StoreDNS() {
				return nil
			}
			cacheFile.SetDisableExpire(options.DNSClientOptions.DisableExpire)
			cacheFile.SetOptimisticTimeout(optimisticTimeout)
			return cacheFile
		},
		Logger: router.logger,
	})
	if options.ReverseMapping {
		router.dnsReverseMapping = common.Must1(freelru.New[netip.Addr, string](1024, maphash.NewHasher[netip.Addr]().Hash32, true))
	}
	return router, nil
}

func (r *Router) Initialize(rules []option.DNSRule) error {
	r.rawRules = append(r.rawRules[:0], rules...)
	newRules, _, _, err := r.buildRules(false)
	if err != nil {
		return err
	}
	closeRules(newRules)
	return nil
}

func (r *Router) Start(stage adapter.StartStage) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateStart:
		monitor.Start("initialize DNS client")
		r.client.Start()
		monitor.Finish()

		monitor.Start("initialize DNS rules")
		newRules, legacyDNSMode, modeFlags, err := r.buildRules(true)
		monitor.Finish()
		if err != nil {
			return err
		}
		r.rulesAccess.Lock()
		if r.closing {
			r.rulesAccess.Unlock()
			closeRules(newRules)
			return nil
		}
		r.rules = newRules
		r.legacyDNSMode = legacyDNSMode
		r.started = true
		r.rulesAccess.Unlock()
		if legacyDNSMode && common.Any(newRules, func(rule adapter.DNSRule) bool { return rule.WithAddressLimit() }) {
			deprecated.Report(r.ctx, deprecated.OptionLegacyDNSAddressFilter)
		}
		if legacyDNSMode && modeFlags.neededFromStrategy {
			deprecated.Report(r.ctx, deprecated.OptionLegacyDNSRuleStrategy)
		}
	}
	return nil
}

func (r *Router) Close() error {
	r.rulesAccess.Lock()
	if r.closing {
		r.rulesAccess.Unlock()
		return nil
	}
	r.closing = true
	runtimeRules := r.rules
	r.rules = nil
	r.rulesAccess.Unlock()
	closeRules(runtimeRules)
	return nil
}

func (r *Router) buildRules(startRules bool) ([]adapter.DNSRule, bool, dnsRuleModeFlags, error) {
	for i, ruleOptions := range r.rawRules {
		err := R.ValidateNoNestedDNSRuleActions(ruleOptions)
		if err != nil {
			return nil, false, dnsRuleModeFlags{}, E.Cause(err, "parse dns rule[", i, "]")
		}
	}
	router := service.FromContext[adapter.Router](r.ctx)
	legacyDNSMode, modeFlags, err := resolveLegacyDNSMode(router, r.rawRules, nil)
	if err != nil {
		return nil, false, dnsRuleModeFlags{}, err
	}
	if !legacyDNSMode {
		err = validateLegacyDNSModeDisabledRules(router, r.rawRules, nil)
		if err != nil {
			return nil, false, dnsRuleModeFlags{}, err
		}
	}
	err = validateEvaluateFakeIPRules(r.rawRules, r.transport)
	if err != nil {
		return nil, false, dnsRuleModeFlags{}, err
	}
	newRules := make([]adapter.DNSRule, 0, len(r.rawRules))
	for i, ruleOptions := range r.rawRules {
		var dnsRule adapter.DNSRule
		dnsRule, err = R.NewDNSRule(r.ctx, r.logger, ruleOptions, true, legacyDNSMode)
		if err != nil {
			closeRules(newRules)
			return nil, false, dnsRuleModeFlags{}, E.Cause(err, "parse dns rule[", i, "]")
		}
		newRules = append(newRules, dnsRule)
	}
	if startRules {
		for i, rule := range newRules {
			err = rule.Start()
			if err != nil {
				closeRules(newRules)
				return nil, false, dnsRuleModeFlags{}, E.Cause(err, "initialize DNS rule[", i, "]")
			}
		}
	}
	return newRules, legacyDNSMode, modeFlags, nil
}

func closeRules(rules []adapter.DNSRule) {
	for _, rule := range rules {
		_ = rule.Close()
	}
}

func (r *Router) ValidateRuleSetMetadataUpdate(tag string, metadata adapter.RuleSetMetadata) error {
	if len(r.rawRules) == 0 {
		return nil
	}
	router := service.FromContext[adapter.Router](r.ctx)
	if router == nil {
		return E.New("router service not found")
	}
	overrides := map[string]adapter.RuleSetMetadata{
		tag: metadata,
	}
	r.rulesAccess.RLock()
	started := r.started
	legacyDNSMode := r.legacyDNSMode
	closing := r.closing
	r.rulesAccess.RUnlock()
	if closing {
		return nil
	}
	if !started {
		candidateLegacyDNSMode, _, err := resolveLegacyDNSMode(router, r.rawRules, overrides)
		if err != nil {
			return err
		}
		if !candidateLegacyDNSMode {
			return validateLegacyDNSModeDisabledRules(router, r.rawRules, overrides)
		}
		return nil
	}
	candidateLegacyDNSMode, flags, err := resolveLegacyDNSMode(router, r.rawRules, overrides)
	if err != nil {
		return err
	}
	if legacyDNSMode {
		if !candidateLegacyDNSMode && flags.disabled {
			err := validateLegacyDNSModeDisabledRules(router, r.rawRules, overrides)
			if err != nil {
				return err
			}
			return E.New(deprecated.OptionLegacyDNSAddressFilter.MessageWithLink())
		}
		return nil
	}
	if candidateLegacyDNSMode {
		return E.New(deprecated.OptionLegacyDNSAddressFilter.MessageWithLink())
	}
	return validateLegacyDNSModeDisabledRules(router, r.rawRules, overrides)
}

func (r *Router) matchDNS(ctx context.Context, rules []adapter.DNSRule, allowFakeIP bool, ruleIndex int, isAddressQuery bool, options *adapter.DNSQueryOptions) (adapter.DNSTransport, adapter.DNSRule, int) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	var currentRuleIndex int
	if ruleIndex != -1 {
		currentRuleIndex = ruleIndex + 1
	}
	for ; currentRuleIndex < len(rules); currentRuleIndex++ {
		currentRule := rules[currentRuleIndex]
		if currentRule.WithAddressLimit() && !isAddressQuery {
			continue
		}
		metadata.ResetRuleCache()
		metadata.DestinationAddressMatchFromResponse = false
		if currentRule.LegacyPreMatch(metadata) {
			if ruleDescription := currentRule.String(); ruleDescription != "" {
				r.logger.DebugContext(ctx, "match[", currentRuleIndex, "] ", currentRule, " => ", currentRule.Action())
			} else {
				r.logger.DebugContext(ctx, "match[", currentRuleIndex, "] => ", currentRule.Action())
			}
			switch action := currentRule.Action().(type) {
			case *R.RuleActionDNSRoute:
				transport, loaded := r.transport.Transport(action.Server)
				if !loaded {
					r.logger.ErrorContext(ctx, "transport not found: ", action.Server)
					continue
				}
				isFakeIP := transport.Type() == C.DNSTypeFakeIP
				if isFakeIP && !allowFakeIP {
					continue
				}
				if action.Strategy != C.DomainStrategyAsIS {
					options.Strategy = action.Strategy
				}
				if isFakeIP || action.DisableCache {
					options.DisableCache = true
				}
				if action.RewriteTTL != nil {
					options.RewriteTTL = action.RewriteTTL
				}
				if action.Timeout > 0 {
					options.Timeout = action.Timeout
				}
				if action.ClientSubnet.IsValid() {
					options.ClientSubnet = action.ClientSubnet
				}
				return transport, currentRule, currentRuleIndex
			case *R.RuleActionDNSRouteOptions:
				if action.Strategy != C.DomainStrategyAsIS {
					options.Strategy = action.Strategy
				}
				if action.DisableCache {
					options.DisableCache = true
				}
				if action.RewriteTTL != nil {
					options.RewriteTTL = action.RewriteTTL
				}
				if action.Timeout > 0 {
					options.Timeout = action.Timeout
				}
				if action.ClientSubnet.IsValid() {
					options.ClientSubnet = action.ClientSubnet
				}
			case *R.RuleActionReject:
				return nil, currentRule, currentRuleIndex
			case *R.RuleActionPredefined:
				return nil, currentRule, currentRuleIndex
			}
		}
	}
	transport := r.transport.Default()
	return transport, nil, -1
}

func (r *Router) applyDNSRouteOptions(options *adapter.DNSQueryOptions, routeOptions R.RuleActionDNSRouteOptions) {
	// Strategy is intentionally skipped here. A non-default DNS rule action strategy
	// forces legacy mode via resolveLegacyDNSMode, so this path is only reachable
	// when strategy remains at its default value.
	if routeOptions.DisableCache {
		options.DisableCache = true
	}
	if routeOptions.DisableOptimisticCache {
		options.DisableOptimisticCache = true
	}
	if routeOptions.RewriteTTL != nil {
		options.RewriteTTL = routeOptions.RewriteTTL
	}
	if routeOptions.Timeout > 0 {
		options.Timeout = routeOptions.Timeout
	}
	if routeOptions.ClientSubnet.IsValid() {
		options.ClientSubnet = routeOptions.ClientSubnet
	}
}

type dnsRouteStatus uint8

const (
	dnsRouteStatusMissing dnsRouteStatus = iota
	dnsRouteStatusSkipped
	dnsRouteStatusResolved
)

func (r *Router) resolveDNSRoute(server string, routeOptions R.RuleActionDNSRouteOptions, allowFakeIP bool, options *adapter.DNSQueryOptions) (adapter.DNSTransport, dnsRouteStatus) {
	transport, loaded := r.transport.Transport(server)
	if !loaded {
		return nil, dnsRouteStatusMissing
	}
	isFakeIP := transport.Type() == C.DNSTypeFakeIP
	if isFakeIP && !allowFakeIP {
		return transport, dnsRouteStatusSkipped
	}
	r.applyDNSRouteOptions(options, routeOptions)
	if isFakeIP {
		options.DisableCache = true
	}
	return transport, dnsRouteStatusResolved
}

func (r *Router) logRuleMatch(ctx context.Context, ruleIndex int, currentRule adapter.DNSRule) {
	if ruleDescription := currentRule.String(); ruleDescription != "" {
		r.logger.DebugContext(ctx, "match[", ruleIndex, "] ", currentRule, " => ", currentRule.Action())
	} else {
		r.logger.DebugContext(ctx, "match[", ruleIndex, "] => ", currentRule.Action())
	}
}

type exchangeWithRulesResult struct {
	response     *mDNS.Msg
	transport    adapter.DNSTransport
	rejectAction *R.RuleActionReject
	err          error
}

const dnsRespondMissingResponseMessage = "respond action requires an evaluated response from a preceding evaluate action"

type dnsRuleWalkState struct {
	ruleIndex          int
	effectiveOptions   adapter.DNSQueryOptions
	evaluatedResponse  *mDNS.Msg
	evaluatedTransport adapter.DNSTransport
}

type dnsPendingExchange struct {
	transport adapter.DNSTransport
	options   adapter.DNSQueryOptions
	evaluate  bool
}

func (r *Router) finalizeExchangeOptions(options adapter.DNSQueryOptions) adapter.DNSQueryOptions {
	if options.Strategy == C.DomainStrategyAsIS {
		options.Strategy = r.defaultDomainStrategy
	}
	return options
}

func (r *Router) walkDNSRules(ctx context.Context, rules []adapter.DNSRule, message *mDNS.Msg, state *dnsRuleWalkState, allowFakeIP bool) (exchangeWithRulesResult, *dnsPendingExchange) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	for ; state.ruleIndex < len(rules); state.ruleIndex++ {
		currentRule := rules[state.ruleIndex]
		metadata.ResetRuleCache()
		metadata.DNSResponse = state.evaluatedResponse
		metadata.DestinationAddressMatchFromResponse = false
		if !currentRule.Match(metadata) {
			continue
		}
		r.logRuleMatch(ctx, state.ruleIndex, currentRule)
		switch action := currentRule.Action().(type) {
		case *R.RuleActionDNSRouteOptions:
			r.applyDNSRouteOptions(&state.effectiveOptions, *action)
		case *R.RuleActionEvaluate:
			queryOptions := state.effectiveOptions
			transport, loaded := r.transport.Transport(action.Server)
			if !loaded {
				r.logger.ErrorContext(ctx, "transport not found: ", action.Server)
				state.evaluatedResponse = nil
				state.evaluatedTransport = nil
				continue
			}
			r.applyDNSRouteOptions(&queryOptions, action.RuleActionDNSRouteOptions)
			return exchangeWithRulesResult{}, &dnsPendingExchange{transport: transport, options: queryOptions, evaluate: true}
		case *R.RuleActionRespond:
			if state.evaluatedResponse == nil {
				return exchangeWithRulesResult{
					err: E.New(dnsRespondMissingResponseMessage),
				}, nil
			}
			return exchangeWithRulesResult{
				response:  state.evaluatedResponse,
				transport: state.evaluatedTransport,
			}, nil
		case *R.RuleActionDNSRoute:
			queryOptions := state.effectiveOptions
			transport, status := r.resolveDNSRoute(action.Server, action.RuleActionDNSRouteOptions, allowFakeIP, &queryOptions)
			switch status {
			case dnsRouteStatusMissing:
				r.logger.ErrorContext(ctx, "transport not found: ", action.Server)
				continue
			case dnsRouteStatusSkipped:
				continue
			}
			return exchangeWithRulesResult{}, &dnsPendingExchange{transport: transport, options: queryOptions}
		case *R.RuleActionReject:
			switch action.Method {
			case C.RuleActionRejectMethodDefault:
				return exchangeWithRulesResult{
					response: &mDNS.Msg{
						MsgHdr: mDNS.MsgHdr{
							Id:       message.Id,
							Rcode:    mDNS.RcodeRefused,
							Response: true,
						},
						Question: []mDNS.Question{message.Question[0]},
					},
					rejectAction: action,
				}, nil
			case C.RuleActionRejectMethodDrop:
				return exchangeWithRulesResult{
					rejectAction: action,
					err:          R.ErrDrop,
				}, nil
			}
		case *R.RuleActionPredefined:
			return exchangeWithRulesResult{
				response: action.Response(message),
			}, nil
		}
	}
	return exchangeWithRulesResult{}, &dnsPendingExchange{transport: r.transport.Default(), options: state.effectiveOptions}
}

func (r *Router) exchangeWithRules(ctx context.Context, rules []adapter.DNSRule, message *mDNS.Msg, options adapter.DNSQueryOptions, allowFakeIP bool) exchangeWithRulesResult {
	state := dnsRuleWalkState{effectiveOptions: options}
	result, pending := r.walkDNSRules(ctx, rules, message, &state, allowFakeIP)
	if pending == nil {
		return result
	}
	return r.resumeExchangeWithRules(ctx, rules, message, &state, allowFakeIP, pending)
}

func (r *Router) resumeExchangeWithRules(ctx context.Context, rules []adapter.DNSRule, message *mDNS.Msg, state *dnsRuleWalkState, allowFakeIP bool, pending *dnsPendingExchange) exchangeWithRulesResult {
	for {
		response, err := r.client.Exchange(adapter.OverrideContext(ctx), pending.transport, message, r.finalizeExchangeOptions(pending.options), nil)
		if !pending.evaluate {
			return exchangeWithRulesResult{
				response:  response,
				transport: pending.transport,
				err:       err,
			}
		}
		if err != nil {
			r.logger.ErrorContext(ctx, E.Cause(err, "exchange failed for ", FormatQuestion(message.Question[0].String())))
			state.evaluatedResponse = nil
			state.evaluatedTransport = nil
		} else {
			state.evaluatedResponse = response
			state.evaluatedTransport = pending.transport
		}
		state.ruleIndex++
		var result exchangeWithRulesResult
		result, pending = r.walkDNSRules(ctx, rules, message, state, allowFakeIP)
		if pending == nil {
			return result
		}
	}
}

func (r *Router) exchangeWithRulesAsync(ctx context.Context, rules []adapter.DNSRule, message *mDNS.Msg, options adapter.DNSQueryOptions, allowFakeIP bool, callback func(result exchangeWithRulesResult)) {
	state := dnsRuleWalkState{effectiveOptions: options}
	result, pending := r.walkDNSRules(ctx, rules, message, &state, allowFakeIP)
	if pending == nil {
		callback(result)
		return
	}
	if pending.evaluate {
		go func() {
			callback(r.resumeExchangeWithRules(ctx, rules, message, &state, allowFakeIP, pending))
		}()
		return
	}
	r.client.ExchangeAsync(adapter.OverrideContext(ctx), pending.transport, message, r.finalizeExchangeOptions(pending.options), nil, func(response *mDNS.Msg, err error) {
		callback(exchangeWithRulesResult{
			response:  response,
			transport: pending.transport,
			err:       err,
		})
	})
}

func (r *Router) resolveLookupStrategy(options adapter.DNSQueryOptions) C.DomainStrategy {
	if options.LookupStrategy != C.DomainStrategyAsIS {
		return options.LookupStrategy
	}
	if options.Strategy != C.DomainStrategyAsIS {
		return options.Strategy
	}
	return r.defaultDomainStrategy
}

func withLookupQueryMetadata(ctx context.Context, qType uint16) context.Context {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.QueryType = qType
	metadata.IPVersion = 0
	switch qType {
	case mDNS.TypeA:
		metadata.IPVersion = 4
	case mDNS.TypeAAAA:
		metadata.IPVersion = 6
	}
	return ctx
}

func filterAddressesByQueryType(addresses []netip.Addr, qType uint16) []netip.Addr {
	switch qType {
	case mDNS.TypeA:
		return common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is4()
		})
	case mDNS.TypeAAAA:
		return common.Filter(addresses, func(address netip.Addr) bool {
			return address.Is6()
		})
	default:
		return addresses
	}
}

func (r *Router) lookupWithRules(ctx context.Context, rules []adapter.DNSRule, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	strategy := r.resolveLookupStrategy(options)
	lookupOptions := options
	if strategy != C.DomainStrategyAsIS {
		lookupOptions.Strategy = strategy
	}
	if strategy == C.DomainStrategyIPv4Only {
		return r.lookupWithRulesType(ctx, rules, domain, mDNS.TypeA, lookupOptions)
	}
	if strategy == C.DomainStrategyIPv6Only {
		return r.lookupWithRulesType(ctx, rules, domain, mDNS.TypeAAAA, lookupOptions)
	}
	var (
		response4 []netip.Addr
		response6 []netip.Addr
	)
	var group task.Group
	group.Append("exchange4", func(ctx context.Context) error {
		result, err := r.lookupWithRulesType(ctx, rules, domain, mDNS.TypeA, lookupOptions)
		response4 = result
		return err
	})
	group.Append("exchange6", func(ctx context.Context) error {
		result, err := r.lookupWithRulesType(ctx, rules, domain, mDNS.TypeAAAA, lookupOptions)
		response6 = result
		return err
	})
	err := group.Run(ctx)
	if len(response4) == 0 && len(response6) == 0 {
		return nil, err
	}
	return sortAddresses(response4, response6, strategy), nil
}

func (r *Router) lookupWithRulesType(ctx context.Context, rules []adapter.DNSRule, domain string, qType uint16, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{{
			Name:   mDNS.Fqdn(domain),
			Qtype:  qType,
			Qclass: mDNS.ClassINET,
		}},
	}
	exchangeResult := r.exchangeWithRules(withLookupQueryMetadata(ctx, qType), rules, request, options, false)
	if exchangeResult.rejectAction != nil {
		return nil, exchangeResult.rejectAction.Error(ctx)
	}
	if exchangeResult.err != nil {
		return nil, exchangeResult.err
	}
	if exchangeResult.response.Rcode != mDNS.RcodeSuccess {
		return nil, RcodeError(exchangeResult.response.Rcode)
	}
	return filterAddressesByQueryType(MessageToAddresses(exchangeResult.response), qType), nil
}

type dnsExchangeContext struct {
	ctx           context.Context
	rules         []adapter.DNSRule
	legacyDNSMode bool
	metadata      *adapter.InboundContext
}

func (r *Router) prepareExchange(ctx context.Context, message *mDNS.Msg) (*dnsExchangeContext, *mDNS.Msg, error) {
	if len(message.Question) != 1 {
		r.logger.WarnContext(ctx, "bad question size: ", len(message.Question))
		return nil, &mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Response: true,
				Rcode:    mDNS.RcodeFormatError,
			},
			Question: message.Question,
		}, nil
	}
	r.rulesAccess.RLock()
	if r.closing {
		r.rulesAccess.RUnlock()
		return nil, nil, E.New("dns router closed")
	}
	rules := r.rules
	legacyDNSMode := r.legacyDNSMode
	r.rulesAccess.RUnlock()
	r.logger.DebugContext(ctx, "exchange ", FormatQuestion(message.Question[0].String()))
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Destination = M.Socksaddr{}
	metadata.QueryType = message.Question[0].Qtype
	metadata.DNSResponse = nil
	metadata.DestinationAddressMatchFromResponse = false
	switch metadata.QueryType {
	case mDNS.TypeA:
		metadata.IPVersion = 4
	case mDNS.TypeAAAA:
		metadata.IPVersion = 6
	}
	metadata.Domain = FqdnToDomain(message.Question[0].Name)
	return &dnsExchangeContext{
		ctx:           ctx,
		rules:         rules,
		legacyDNSMode: legacyDNSMode,
		metadata:      metadata,
	}, nil, nil
}

func (r *Router) recordReverseMapping(message *mDNS.Msg, response *mDNS.Msg, transport adapter.DNSTransport) {
	if r.dnsReverseMapping != nil && len(message.Question) > 0 && response != nil && len(response.Answer) > 0 {
		if transport == nil || transport.Type() != C.DNSTypeFakeIP {
			for _, answer := range response.Answer {
				switch record := answer.(type) {
				case *mDNS.A:
					r.dnsReverseMapping.AddWithLifetime(M.AddrFromIP(record.A), FqdnToDomain(record.Hdr.Name), time.Duration(record.Hdr.Ttl)*time.Second)
				case *mDNS.AAAA:
					r.dnsReverseMapping.AddWithLifetime(M.AddrFromIP(record.AAAA), FqdnToDomain(record.Hdr.Name), time.Duration(record.Hdr.Ttl)*time.Second)
				}
			}
		}
	}
}

func (r *Router) exchangeLegacy(ctx context.Context, exchangeCtx *dnsExchangeContext, message *mDNS.Msg, options adapter.DNSQueryOptions) (*mDNS.Msg, adapter.DNSTransport, error) {
	var (
		transport adapter.DNSTransport
		rule      adapter.DNSRule
		ruleIndex int
	)
	ruleIndex = -1
	for {
		dnsCtx := adapter.OverrideContext(ctx)
		dnsOptions := options
		transport, rule, ruleIndex = r.matchDNS(ctx, exchangeCtx.rules, true, ruleIndex, isAddressQuery(message), &dnsOptions)
		if rule != nil {
			switch action := rule.Action().(type) {
			case *R.RuleActionReject:
				switch action.Method {
				case C.RuleActionRejectMethodDefault:
					return &mDNS.Msg{
						MsgHdr: mDNS.MsgHdr{
							Id:       message.Id,
							Rcode:    mDNS.RcodeRefused,
							Response: true,
						},
						Question: []mDNS.Question{message.Question[0]},
					}, nil, nil
				case C.RuleActionRejectMethodDrop:
					return nil, nil, R.ErrDrop
				}
			case *R.RuleActionPredefined:
				return action.Response(message), nil, nil
			}
		}
		responseCheck := addressLimitResponseCheck(rule, exchangeCtx.metadata)
		response, err := r.client.Exchange(dnsCtx, transport, message, r.finalizeExchangeOptions(dnsOptions), responseCheck)
		var rejected bool
		if err != nil {
			if errors.Is(err, ErrResponseRejectedCached) {
				rejected = true
				r.logger.DebugContext(ctx, E.Cause(err, "response rejected for ", FormatQuestion(message.Question[0].String())), " (cached)")
			} else if errors.Is(err, ErrResponseRejected) {
				rejected = true
				r.logger.DebugContext(ctx, E.Cause(err, "response rejected for ", FormatQuestion(message.Question[0].String())))
			} else if len(message.Question) > 0 {
				r.logger.ErrorContext(ctx, E.Cause(err, "exchange failed for ", FormatQuestion(message.Question[0].String())))
			} else {
				r.logger.ErrorContext(ctx, E.Cause(err, "exchange failed for <empty query>"))
			}
		}
		if responseCheck != nil && rejected {
			continue
		}
		return response, transport, err
	}
}

func (r *Router) Exchange(ctx context.Context, message *mDNS.Msg, options adapter.DNSQueryOptions) (*mDNS.Msg, error) {
	exchangeCtx, earlyResponse, err := r.prepareExchange(ctx, message)
	if exchangeCtx == nil {
		return earlyResponse, err
	}
	ctx = exchangeCtx.ctx
	var (
		response  *mDNS.Msg
		transport adapter.DNSTransport
	)
	if options.Transport != nil {
		transport = options.Transport
		response, err = r.client.Exchange(ctx, transport, message, r.finalizeExchangeOptions(options), nil)
	} else if !exchangeCtx.legacyDNSMode {
		exchangeResult := r.exchangeWithRules(ctx, exchangeCtx.rules, message, options, true)
		response, transport, err = exchangeResult.response, exchangeResult.transport, exchangeResult.err
	} else {
		response, transport, err = r.exchangeLegacy(ctx, exchangeCtx, message, options)
	}
	if err != nil {
		return nil, err
	}
	r.recordReverseMapping(message, response, transport)
	return response, nil
}

func (r *Router) ExchangeAsync(ctx context.Context, message *mDNS.Msg, options adapter.DNSQueryOptions, callback func(response *mDNS.Msg, err error)) {
	exchangeCtx, earlyResponse, err := r.prepareExchange(ctx, message)
	if exchangeCtx == nil {
		callback(earlyResponse, err)
		return
	}
	ctx = exchangeCtx.ctx
	if options.Transport != nil {
		transport := options.Transport
		r.client.ExchangeAsync(ctx, transport, message, r.finalizeExchangeOptions(options), nil, func(response *mDNS.Msg, exchangeErr error) {
			r.finishExchangeAsync(message, transport, response, exchangeErr, callback)
		})
	} else if !exchangeCtx.legacyDNSMode {
		r.exchangeWithRulesAsync(ctx, exchangeCtx.rules, message, options, true, func(result exchangeWithRulesResult) {
			r.finishExchangeAsync(message, result.transport, result.response, result.err, callback)
		})
	} else {
		go func() {
			response, transport, exchangeErr := r.exchangeLegacy(ctx, exchangeCtx, message, options)
			r.finishExchangeAsync(message, transport, response, exchangeErr, callback)
		}()
	}
}

func (r *Router) finishExchangeAsync(message *mDNS.Msg, transport adapter.DNSTransport, response *mDNS.Msg, err error, callback func(response *mDNS.Msg, err error)) {
	if err != nil {
		callback(nil, err)
		return
	}
	r.recordReverseMapping(message, response, transport)
	callback(response, nil)
}

func (r *Router) Lookup(ctx context.Context, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	r.rulesAccess.RLock()
	if r.closing {
		r.rulesAccess.RUnlock()
		return nil, E.New("dns router closed")
	}
	rules := r.rules
	legacyDNSMode := r.legacyDNSMode
	r.rulesAccess.RUnlock()
	var (
		responseAddrs []netip.Addr
		err           error
	)
	printResult := func() {
		if err == nil && len(responseAddrs) == 0 {
			err = E.New("empty result")
		}
		if err != nil {
			if errors.Is(err, ErrResponseRejectedCached) {
				r.logger.DebugContext(ctx, "response rejected for ", domain, " (cached)")
			} else if errors.Is(err, ErrResponseRejected) {
				r.logger.DebugContext(ctx, "response rejected for ", domain)
			} else if R.IsRejected(err) {
				r.logger.DebugContext(ctx, "lookup rejected for ", domain)
			} else if errors.Is(err, ErrNotCached) {
				r.logger.DebugContext(ctx, "cache-only lookup missed for ", domain)
			} else {
				r.logger.ErrorContext(ctx, E.Cause(err, "lookup failed for ", domain))
			}
		}
		if err != nil {
			err = E.Cause(err, "lookup ", domain)
		}
	}
	r.logger.DebugContext(ctx, "lookup domain ", domain)
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Destination = M.Socksaddr{}
	metadata.Domain = FqdnToDomain(domain)
	metadata.DNSResponse = nil
	metadata.DestinationAddressMatchFromResponse = false
	if options.Transport != nil {
		transport := options.Transport
		if options.Strategy == C.DomainStrategyAsIS {
			options.Strategy = r.defaultDomainStrategy
		}
		responseAddrs, err = r.client.Lookup(ctx, transport, domain, options, nil)
	} else if !legacyDNSMode {
		responseAddrs, err = r.lookupWithRules(ctx, rules, domain, options)
	} else {
		var (
			transport adapter.DNSTransport
			rule      adapter.DNSRule
			ruleIndex int
		)
		ruleIndex = -1
		for {
			dnsCtx := adapter.OverrideContext(ctx)
			dnsOptions := options
			transport, rule, ruleIndex = r.matchDNS(ctx, rules, false, ruleIndex, true, &dnsOptions)
			if rule != nil {
				switch action := rule.Action().(type) {
				case *R.RuleActionReject:
					return nil, &R.RejectedError{Cause: action.Error(ctx)}
				case *R.RuleActionPredefined:
					responseAddrs = nil
					if action.Rcode != mDNS.RcodeSuccess {
						err = RcodeError(action.Rcode)
					} else {
						err = nil
						for _, answer := range action.Answer {
							switch record := answer.(type) {
							case *mDNS.A:
								responseAddrs = append(responseAddrs, M.AddrFromIP(record.A))
							case *mDNS.AAAA:
								responseAddrs = append(responseAddrs, M.AddrFromIP(record.AAAA))
							}
						}
					}
					goto response
				}
			}
			responseCheck := addressLimitResponseCheck(rule, metadata)
			if dnsOptions.Strategy == C.DomainStrategyAsIS {
				dnsOptions.Strategy = r.defaultDomainStrategy
			}
			responseAddrs, err = r.client.Lookup(dnsCtx, transport, domain, dnsOptions, responseCheck)
			if responseCheck == nil || err == nil {
				break
			}
			printResult()
		}
	}
response:
	printResult()
	if len(responseAddrs) > 0 {
		r.logger.InfoContext(ctx, "lookup succeed for ", domain, ": ", strings.Join(F.MapToString(responseAddrs), " "))
	}
	return responseAddrs, err
}

func isAddressQuery(message *mDNS.Msg) bool {
	for _, question := range message.Question {
		if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA || question.Qtype == mDNS.TypeHTTPS {
			return true
		}
	}
	return false
}

func addressLimitResponseCheck(rule adapter.DNSRule, metadata *adapter.InboundContext) func(response *mDNS.Msg) bool {
	if rule == nil || !rule.WithAddressLimit() {
		return nil
	}
	responseMetadata := *metadata
	return func(response *mDNS.Msg) bool {
		checkMetadata := responseMetadata
		return rule.MatchAddressLimit(&checkMetadata, response)
	}
}

func (r *Router) ClearCache() {
	r.client.ClearCache()
	if r.platformInterface != nil {
		r.platformInterface.ClearDNSCache()
	}
	if r.dnsReverseMapping != nil {
		r.dnsReverseMapping.Purge()
	}
}

func (r *Router) LookupReverseMapping(ip netip.Addr) (string, bool) {
	if r.dnsReverseMapping == nil {
		return "", false
	}
	domain, loaded := r.dnsReverseMapping.Get(ip)
	return domain, loaded
}

func (r *Router) ResetNetwork() {
	r.ClearCache()
	for _, transport := range r.transport.Transports() {
		transport.Reset()
	}
}

func defaultRuleNeedsLegacyDNSModeFromAddressFilter(rule option.DefaultDNSRule) bool {
	if rule.RuleSetIPCIDRAcceptEmpty { //nolint:staticcheck
		return true
	}
	return !rule.MatchResponse && (rule.IPAcceptAny || len(rule.IPCIDR) > 0 || rule.IPIsPrivate)
}

func hasResponseMatchFields(rule option.DefaultDNSRule) bool {
	return rule.ResponseRcode != nil ||
		len(rule.ResponseAnswer) > 0 ||
		len(rule.ResponseNs) > 0 ||
		len(rule.ResponseExtra) > 0
}

func defaultRuleDisablesLegacyDNSMode(rule option.DefaultDNSRule) bool {
	return rule.MatchResponse ||
		hasResponseMatchFields(rule) ||
		rule.Action == C.RuleActionTypeEvaluate ||
		rule.Action == C.RuleActionTypeRespond ||
		rule.IPVersion > 0 ||
		len(rule.QueryType) > 0
}

type dnsRuleModeFlags struct {
	disabled           bool
	needed             bool
	neededFromStrategy bool
}

func (f *dnsRuleModeFlags) merge(other dnsRuleModeFlags) {
	f.disabled = f.disabled || other.disabled
	f.needed = f.needed || other.needed
	f.neededFromStrategy = f.neededFromStrategy || other.neededFromStrategy
}

func resolveLegacyDNSMode(router adapter.Router, rules []option.DNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (bool, dnsRuleModeFlags, error) {
	flags, err := dnsRuleModeRequirements(router, rules, metadataOverrides)
	if err != nil {
		return false, flags, err
	}
	if flags.disabled && flags.neededFromStrategy {
		return false, flags, E.New(deprecated.OptionLegacyDNSRuleStrategy.MessageWithLink())
	}
	if flags.disabled {
		return false, flags, nil
	}
	return flags.needed, flags, nil
}

func dnsRuleModeRequirements(router adapter.Router, rules []option.DNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (dnsRuleModeFlags, error) {
	var flags dnsRuleModeFlags
	for i, rule := range rules {
		ruleFlags, err := dnsRuleModeRequirementsInRule(router, rule, metadataOverrides)
		if err != nil {
			return dnsRuleModeFlags{}, E.Cause(err, "dns rule[", i, "]")
		}
		flags.merge(ruleFlags)
	}
	return flags, nil
}

func dnsRuleModeRequirementsInRule(router adapter.Router, rule option.DNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (dnsRuleModeFlags, error) {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return dnsRuleModeRequirementsInDefaultRule(router, rule.DefaultOptions, metadataOverrides)
	case C.RuleTypeLogical:
		flags := dnsRuleModeFlags{
			disabled: dnsRuleActionType(rule) == C.RuleActionTypeEvaluate ||
				dnsRuleActionType(rule) == C.RuleActionTypeRespond ||
				dnsRuleActionDisablesLegacyDNSMode(rule.LogicalOptions.DNSRuleAction),
			neededFromStrategy: dnsRuleActionHasStrategy(rule.LogicalOptions.DNSRuleAction),
		}
		flags.needed = flags.neededFromStrategy
		for i, subRule := range rule.LogicalOptions.Rules {
			subFlags, err := dnsRuleModeRequirementsInRule(router, subRule, metadataOverrides)
			if err != nil {
				return dnsRuleModeFlags{}, E.Cause(err, "sub rule[", i, "]")
			}
			flags.merge(subFlags)
		}
		return flags, nil
	default:
		return dnsRuleModeFlags{}, nil
	}
}

func dnsRuleModeRequirementsInDefaultRule(router adapter.Router, rule option.DefaultDNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (dnsRuleModeFlags, error) {
	flags := dnsRuleModeFlags{
		disabled:           defaultRuleDisablesLegacyDNSMode(rule) || dnsRuleActionDisablesLegacyDNSMode(rule.DNSRuleAction),
		neededFromStrategy: dnsRuleActionHasStrategy(rule.DNSRuleAction),
	}
	flags.needed = defaultRuleNeedsLegacyDNSModeFromAddressFilter(rule) || flags.neededFromStrategy
	if len(rule.RuleSet) == 0 {
		return flags, nil
	}
	if router == nil {
		return dnsRuleModeFlags{}, E.New("router service not found")
	}
	for _, tag := range rule.RuleSet {
		metadata, err := lookupDNSRuleSetMetadata(router, tag, metadataOverrides)
		if err != nil {
			return dnsRuleModeFlags{}, err
		}
		// ip_version is not a headless-rule item, so ContainsIPVersionRule is intentionally absent.
		flags.disabled = flags.disabled || metadata.ContainsDNSQueryTypeRule
		if !rule.RuleSetIPCIDRMatchSource && metadata.ContainsIPCIDRRule {
			flags.needed = true
		}
	}
	return flags, nil
}

func lookupDNSRuleSetMetadata(router adapter.Router, tag string, metadataOverrides map[string]adapter.RuleSetMetadata) (adapter.RuleSetMetadata, error) {
	if metadataOverrides != nil {
		if metadata, loaded := metadataOverrides[tag]; loaded {
			return metadata, nil
		}
	}
	ruleSet, loaded := router.RuleSet(tag)
	if !loaded {
		return adapter.RuleSetMetadata{}, E.New("rule-set not found: ", tag)
	}
	return ruleSet.Metadata(), nil
}

func validateLegacyDNSModeDisabledRules(router adapter.Router, rules []option.DNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) error {
	var seenEvaluate bool
	for i, rule := range rules {
		requiresPriorEvaluate, err := validateLegacyDNSModeDisabledRuleTree(router, rule, metadataOverrides)
		if err != nil {
			return E.Cause(err, "validate dns rule[", i, "]")
		}
		if requiresPriorEvaluate && !seenEvaluate {
			return E.New("dns rule[", i, "]: response-based matching requires a preceding evaluate action")
		}
		if dnsRuleActionType(rule) == C.RuleActionTypeEvaluate {
			seenEvaluate = true
		}
	}
	return nil
}

func validateEvaluateFakeIPRules(rules []option.DNSRule, transportManager adapter.DNSTransportManager) error {
	if transportManager == nil {
		return nil
	}
	for i, rule := range rules {
		if dnsRuleActionType(rule) != C.RuleActionTypeEvaluate {
			continue
		}
		server := dnsRuleActionServer(rule)
		if server == "" {
			continue
		}
		transport, loaded := transportManager.Transport(server)
		if !loaded || transport.Type() != C.DNSTypeFakeIP {
			continue
		}
		return E.New("dns rule[", i, "]: evaluate action cannot use fakeip server: ", server)
	}
	return nil
}

func validateLegacyDNSModeDisabledRuleTree(router adapter.Router, rule option.DNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (bool, error) {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return validateLegacyDNSModeDisabledDefaultRule(router, rule.DefaultOptions, metadataOverrides)
	case C.RuleTypeLogical:
		requiresPriorEvaluate := dnsRuleActionType(rule) == C.RuleActionTypeRespond
		for i, subRule := range rule.LogicalOptions.Rules {
			subRequiresPriorEvaluate, err := validateLegacyDNSModeDisabledRuleTree(router, subRule, metadataOverrides)
			if err != nil {
				return false, E.Cause(err, "sub rule[", i, "]")
			}
			requiresPriorEvaluate = requiresPriorEvaluate || subRequiresPriorEvaluate
		}
		return requiresPriorEvaluate, nil
	default:
		return false, nil
	}
}

func validateLegacyDNSModeDisabledDefaultRule(router adapter.Router, rule option.DefaultDNSRule, metadataOverrides map[string]adapter.RuleSetMetadata) (bool, error) {
	hasResponseRecords := hasResponseMatchFields(rule)
	if (hasResponseRecords || len(rule.IPCIDR) > 0 || rule.IPIsPrivate || rule.IPAcceptAny) && !rule.MatchResponse {
		return false, E.New("Response Match Fields (ip_cidr, ip_is_private, ip_accept_any, response_rcode, response_answer, response_ns, response_extra) require match_response to be enabled")
	}
	// rule_set entries are only rejected when every referenced set is pure-IP;
	// mixed sets still fall through because their non-IP branches remain matchable
	// before a DNS response is available.
	if !rule.MatchResponse && len(rule.RuleSet) > 0 {
		for _, tag := range rule.RuleSet {
			metadata, err := lookupDNSRuleSetMetadata(router, tag, metadataOverrides)
			if err != nil {
				return false, err
			}
			if metadata.ContainsIPCIDRRule && !metadata.ContainsNonIPCIDRRule {
				return false, E.New(deprecated.OptionLegacyDNSAddressFilter.MessageWithLink())
			}
		}
	}
	if rule.RuleSetIPCIDRAcceptEmpty { //nolint:staticcheck
		return false, E.New(deprecated.OptionRuleSetIPCIDRAcceptEmpty.MessageWithLink())
	}
	return rule.MatchResponse || rule.Action == C.RuleActionTypeRespond, nil
}

func dnsRuleActionDisablesLegacyDNSMode(action option.DNSRuleAction) bool {
	switch action.Action {
	case "", C.RuleActionTypeRoute, C.RuleActionTypeEvaluate:
		return action.RouteOptions.DisableOptimisticCache
	case C.RuleActionTypeRouteOptions:
		return action.RouteOptionsOptions.DisableOptimisticCache
	default:
		return false
	}
}

func dnsRuleActionHasStrategy(action option.DNSRuleAction) bool {
	switch action.Action {
	case "", C.RuleActionTypeRoute, C.RuleActionTypeEvaluate:
		return C.DomainStrategy(action.RouteOptions.Strategy) != C.DomainStrategyAsIS
	case C.RuleActionTypeRouteOptions:
		return C.DomainStrategy(action.RouteOptionsOptions.Strategy) != C.DomainStrategyAsIS
	default:
		return false
	}
}

func dnsRuleActionType(rule option.DNSRule) string {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		if rule.DefaultOptions.Action == "" {
			return C.RuleActionTypeRoute
		}
		return rule.DefaultOptions.Action
	case C.RuleTypeLogical:
		if rule.LogicalOptions.Action == "" {
			return C.RuleActionTypeRoute
		}
		return rule.LogicalOptions.Action
	default:
		return ""
	}
}

func dnsRuleActionServer(rule option.DNSRule) string {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return rule.DefaultOptions.RouteOptions.Server
	case C.RuleTypeLogical:
		return rule.LogicalOptions.RouteOptions.Server
	default:
		return ""
	}
}

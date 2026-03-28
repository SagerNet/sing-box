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
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSRouter = (*Router)(nil)

type dnsRuleSetCallback struct {
	ruleSet adapter.RuleSet
	element *list.Element[adapter.RuleSetUpdateCallback]
}

type Router struct {
	ctx                             context.Context
	logger                          logger.ContextLogger
	transport                       adapter.DNSTransportManager
	outbound                        adapter.OutboundManager
	client                          adapter.DNSClient
	rawRules                        []option.DNSRule
	rules                           []adapter.DNSRule
	defaultDomainStrategy           C.DomainStrategy
	dnsReverseMapping               freelru.Cache[netip.Addr, string]
	platformInterface               adapter.PlatformInterface
	legacyDNSMode                   bool
	rulesAccess                     sync.RWMutex
	closing                         bool
	ruleSetCallbacks                []dnsRuleSetCallback
	runtimeRuleError                error
	addressFilterDeprecatedReported bool
	ruleStrategyDeprecatedReported  bool
}

func NewRouter(ctx context.Context, logFactory log.Factory, options option.DNSOptions) *Router {
	router := &Router{
		ctx:                   ctx,
		logger:                logFactory.NewLogger("dns"),
		transport:             service.FromContext[adapter.DNSTransportManager](ctx),
		outbound:              service.FromContext[adapter.OutboundManager](ctx),
		rawRules:              make([]option.DNSRule, 0, len(options.Rules)),
		rules:                 make([]adapter.DNSRule, 0, len(options.Rules)),
		defaultDomainStrategy: C.DomainStrategy(options.Strategy),
	}
	router.client = NewClient(ClientOptions{
		DisableCache:     options.DNSClientOptions.DisableCache,
		DisableExpire:    options.DNSClientOptions.DisableExpire,
		IndependentCache: options.DNSClientOptions.IndependentCache,
		CacheCapacity:    options.DNSClientOptions.CacheCapacity,
		ClientSubnet:     options.DNSClientOptions.ClientSubnet.Build(netip.Prefix{}),
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
		Logger: router.logger,
	})
	if options.ReverseMapping {
		router.dnsReverseMapping = common.Must1(freelru.NewSharded[netip.Addr, string](1024, maphash.NewHasher[netip.Addr]().Hash32))
	}
	return router
}

func (r *Router) Initialize(rules []option.DNSRule) error {
	r.rawRules = append(r.rawRules[:0], rules...)
	newRules, _, err := r.buildRules(false)
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
		err := r.rebuildRules(true)
		monitor.Finish()
		if err != nil {
			return err
		}
		monitor.Start("register DNS rule-set callbacks")
		err = r.registerRuleSetCallbacks()
		monitor.Finish()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) Close() error {
	monitor := taskmonitor.New(r.logger, C.StopTimeout)
	r.rulesAccess.Lock()
	r.closing = true
	callbacks := r.ruleSetCallbacks
	r.ruleSetCallbacks = nil
	runtimeRules := r.rules
	r.rules = nil
	r.runtimeRuleError = nil
	for _, callback := range callbacks {
		callback.ruleSet.UnregisterCallback(callback.element)
	}
	r.rulesAccess.Unlock()
	var err error
	for i, rule := range runtimeRules {
		monitor.Start("close dns rule[", i, "]")
		err = E.Append(err, rule.Close(), func(err error) error {
			return E.Cause(err, "close dns rule[", i, "]")
		})
		monitor.Finish()
	}
	return err
}

func (r *Router) rebuildRules(startRules bool) error {
	if r.isClosing() {
		return nil
	}
	newRules, legacyDNSMode, err := r.buildRules(startRules)
	if err != nil {
		if r.isClosing() {
			return nil
		}
		return err
	}
	shouldReportAddressFilterDeprecated := startRules &&
		legacyDNSMode &&
		!r.addressFilterDeprecatedReported &&
		common.Any(newRules, func(rule adapter.DNSRule) bool { return rule.WithAddressLimit() })
	shouldReportRuleStrategyDeprecated := startRules &&
		legacyDNSMode &&
		!r.ruleStrategyDeprecatedReported &&
		hasDNSRuleActionStrategy(r.rawRules)
	r.rulesAccess.Lock()
	if r.closing {
		r.rulesAccess.Unlock()
		closeRules(newRules)
		return nil
	}
	oldRules := r.rules
	r.rules = newRules
	r.legacyDNSMode = legacyDNSMode
	r.runtimeRuleError = nil
	if shouldReportAddressFilterDeprecated {
		r.addressFilterDeprecatedReported = true
	}
	if shouldReportRuleStrategyDeprecated {
		r.ruleStrategyDeprecatedReported = true
	}
	r.rulesAccess.Unlock()
	closeRules(oldRules)
	if shouldReportAddressFilterDeprecated {
		deprecated.Report(r.ctx, deprecated.OptionLegacyDNSAddressFilter)
	}
	if shouldReportRuleStrategyDeprecated {
		deprecated.Report(r.ctx, deprecated.OptionLegacyDNSRuleStrategy)
	}
	return nil
}

func (r *Router) isClosing() bool {
	r.rulesAccess.RLock()
	defer r.rulesAccess.RUnlock()
	return r.closing
}

func (r *Router) buildRules(startRules bool) ([]adapter.DNSRule, bool, error) {
	for i, ruleOptions := range r.rawRules {
		err := R.ValidateNoNestedDNSRuleActions(ruleOptions)
		if err != nil {
			return nil, false, E.Cause(err, "parse dns rule[", i, "]")
		}
	}
	router := service.FromContext[adapter.Router](r.ctx)
	legacyDNSMode, err := resolveLegacyDNSMode(router, r.rawRules)
	if err != nil {
		return nil, false, err
	}
	if !legacyDNSMode {
		err = validateLegacyDNSModeDisabledRules(r.rawRules)
		if err != nil {
			return nil, false, err
		}
	}
	newRules := make([]adapter.DNSRule, 0, len(r.rawRules))
	for i, ruleOptions := range r.rawRules {
		dnsRule, err := R.NewDNSRule(r.ctx, r.logger, ruleOptions, true, legacyDNSMode)
		if err != nil {
			closeRules(newRules)
			return nil, false, E.Cause(err, "parse dns rule[", i, "]")
		}
		newRules = append(newRules, dnsRule)
	}
	if startRules {
		for i, rule := range newRules {
			err := rule.Start()
			if err != nil {
				closeRules(newRules)
				return nil, false, E.Cause(err, "initialize DNS rule[", i, "]")
			}
		}
	}
	return newRules, legacyDNSMode, nil
}

func closeRules(rules []adapter.DNSRule) {
	for _, rule := range rules {
		_ = rule.Close()
	}
}

func (r *Router) registerRuleSetCallbacks() error {
	tags := referencedDNSRuleSetTags(r.rawRules)
	if len(tags) == 0 {
		return nil
	}
	r.rulesAccess.RLock()
	if len(r.ruleSetCallbacks) > 0 {
		r.rulesAccess.RUnlock()
		return nil
	}
	r.rulesAccess.RUnlock()
	router := service.FromContext[adapter.Router](r.ctx)
	if router == nil {
		return E.New("router service not found")
	}
	callbacks := make([]dnsRuleSetCallback, 0, len(tags))
	for _, tag := range tags {
		ruleSet, loaded := router.RuleSet(tag)
		if !loaded {
			for _, callback := range callbacks {
				callback.ruleSet.UnregisterCallback(callback.element)
			}
			return E.New("rule-set not found: ", tag)
		}
		element := ruleSet.RegisterCallback(func(adapter.RuleSet) {
			err := r.rebuildRules(true)
			if err != nil {
				r.rulesAccess.Lock()
				r.runtimeRuleError = err
				r.rulesAccess.Unlock()
				r.logger.Error(E.Cause(err, "rebuild DNS rules after rule-set update"))
			}
		})
		callbacks = append(callbacks, dnsRuleSetCallback{
			ruleSet: ruleSet,
			element: element,
		})
	}
	r.rulesAccess.Lock()
	if len(r.ruleSetCallbacks) == 0 {
		r.ruleSetCallbacks = callbacks
		callbacks = nil
	}
	r.rulesAccess.Unlock()
	for _, callback := range callbacks {
		callback.ruleSet.UnregisterCallback(callback.element)
	}
	return nil
}

func (r *Router) matchDNS(ctx context.Context, allowFakeIP bool, ruleIndex int, isAddressQuery bool, options *adapter.DNSQueryOptions) (adapter.DNSTransport, adapter.DNSRule, int) {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	var currentRuleIndex int
	if ruleIndex != -1 {
		currentRuleIndex = ruleIndex + 1
	}
	for ; currentRuleIndex < len(r.rules); currentRuleIndex++ {
		currentRule := r.rules[currentRuleIndex]
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
	if routeOptions.DisableCache {
		options.DisableCache = true
	}
	if routeOptions.RewriteTTL != nil {
		options.RewriteTTL = routeOptions.RewriteTTL
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

func (r *Router) resolveDNSRoute(action *R.RuleActionDNSRoute, allowFakeIP bool, options *adapter.DNSQueryOptions) (adapter.DNSTransport, dnsRouteStatus) {
	transport, loaded := r.transport.Transport(action.Server)
	if !loaded {
		return nil, dnsRouteStatusMissing
	}
	isFakeIP := transport.Type() == C.DNSTypeFakeIP
	if isFakeIP && !allowFakeIP {
		return transport, dnsRouteStatusSkipped
	}
	r.applyDNSRouteOptions(options, action.RuleActionDNSRouteOptions)
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

func (r *Router) exchangeWithRules(ctx context.Context, message *mDNS.Msg, options adapter.DNSQueryOptions, allowFakeIP bool) exchangeWithRulesResult {
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil {
		panic("no context")
	}
	effectiveOptions := options
	var savedResponse *mDNS.Msg
	for currentRuleIndex, currentRule := range r.rules {
		metadata.ResetRuleCache()
		metadata.DNSResponse = savedResponse
		metadata.DestinationAddressMatchFromResponse = false
		if !currentRule.Match(metadata) {
			continue
		}
		r.logRuleMatch(ctx, currentRuleIndex, currentRule)
		switch action := currentRule.Action().(type) {
		case *R.RuleActionDNSRouteOptions:
			r.applyDNSRouteOptions(&effectiveOptions, *action)
		case *R.RuleActionEvaluate:
			queryOptions := effectiveOptions
			transport, status := r.resolveDNSRoute(&R.RuleActionDNSRoute{
				Server:                    action.Server,
				RuleActionDNSRouteOptions: action.RuleActionDNSRouteOptions,
			}, allowFakeIP, &queryOptions)
			switch status {
			case dnsRouteStatusMissing:
				r.logger.ErrorContext(ctx, "transport not found: ", action.Server)
				savedResponse = nil
				continue
			case dnsRouteStatusSkipped:
				continue
			}
			exchangeOptions := queryOptions
			if exchangeOptions.Strategy == C.DomainStrategyAsIS {
				exchangeOptions.Strategy = r.defaultDomainStrategy
			}
			response, err := r.client.Exchange(adapter.OverrideContext(ctx), transport, message, exchangeOptions, nil)
			if err != nil {
				r.logger.ErrorContext(ctx, E.Cause(err, "exchange failed for ", FormatQuestion(message.Question[0].String())))
				savedResponse = nil
				continue
			}
			savedResponse = response
		case *R.RuleActionDNSRoute:
			queryOptions := effectiveOptions
			transport, status := r.resolveDNSRoute(action, allowFakeIP, &queryOptions)
			switch status {
			case dnsRouteStatusMissing:
				r.logger.ErrorContext(ctx, "transport not found: ", action.Server)
				continue
			case dnsRouteStatusSkipped:
				continue
			}
			exchangeOptions := queryOptions
			if exchangeOptions.Strategy == C.DomainStrategyAsIS {
				exchangeOptions.Strategy = r.defaultDomainStrategy
			}
			response, err := r.client.Exchange(adapter.OverrideContext(ctx), transport, message, exchangeOptions, nil)
			return exchangeWithRulesResult{
				response:  response,
				transport: transport,
				err:       err,
			}
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
				}
			case C.RuleActionRejectMethodDrop:
				return exchangeWithRulesResult{
					rejectAction: action,
					err:          tun.ErrDrop,
				}
			}
		case *R.RuleActionPredefined:
			return exchangeWithRulesResult{
				response: action.Response(message),
			}
		}
	}
	queryOptions := effectiveOptions
	transport := r.transport.Default()
	exchangeOptions := queryOptions
	if exchangeOptions.Strategy == C.DomainStrategyAsIS {
		exchangeOptions.Strategy = r.defaultDomainStrategy
	}
	response, err := r.client.Exchange(adapter.OverrideContext(ctx), transport, message, exchangeOptions, nil)
	return exchangeWithRulesResult{
		response:  response,
		transport: transport,
		err:       err,
	}
}

type lookupWithRulesResponse struct {
	addresses []netip.Addr
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

func lookupStrategyAllowsQueryType(strategy C.DomainStrategy, qType uint16) bool {
	switch strategy {
	case C.DomainStrategyIPv4Only:
		return qType == mDNS.TypeA
	case C.DomainStrategyIPv6Only:
		return qType == mDNS.TypeAAAA
	default:
		return true
	}
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

func (r *Router) lookupWithRules(ctx context.Context, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	strategy := r.resolveLookupStrategy(options)
	lookupOptions := options
	if strategy != C.DomainStrategyAsIS {
		lookupOptions.Strategy = strategy
	}
	if strategy == C.DomainStrategyIPv4Only {
		response, err := r.lookupWithRulesType(ctx, domain, mDNS.TypeA, lookupOptions)
		return response.addresses, err
	}
	if strategy == C.DomainStrategyIPv6Only {
		response, err := r.lookupWithRulesType(ctx, domain, mDNS.TypeAAAA, lookupOptions)
		return response.addresses, err
	}
	var (
		response4 lookupWithRulesResponse
		response6 lookupWithRulesResponse
	)
	var group task.Group
	group.Append("exchange4", func(ctx context.Context) error {
		result, err := r.lookupWithRulesType(ctx, domain, mDNS.TypeA, lookupOptions)
		response4 = result
		return err
	})
	group.Append("exchange6", func(ctx context.Context) error {
		result, err := r.lookupWithRulesType(ctx, domain, mDNS.TypeAAAA, lookupOptions)
		response6 = result
		return err
	})
	err := group.Run(ctx)
	if len(response4.addresses) == 0 && len(response6.addresses) == 0 {
		return nil, err
	}
	return sortAddresses(response4.addresses, response6.addresses, strategy), nil
}

func (r *Router) lookupWithRulesType(ctx context.Context, domain string, qType uint16, options adapter.DNSQueryOptions) (lookupWithRulesResponse, error) {
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{{
			Name:   mDNS.Fqdn(FqdnToDomain(domain)),
			Qtype:  qType,
			Qclass: mDNS.ClassINET,
		}},
	}
	exchangeResult := r.exchangeWithRules(withLookupQueryMetadata(ctx, qType), request, options, false)
	result := lookupWithRulesResponse{}
	if exchangeResult.rejectAction != nil {
		return result, exchangeResult.rejectAction.Error(ctx)
	}
	if exchangeResult.err != nil {
		return result, exchangeResult.err
	}
	if exchangeResult.response.Rcode != mDNS.RcodeSuccess {
		return result, RcodeError(exchangeResult.response.Rcode)
	}
	if !lookupStrategyAllowsQueryType(r.resolveLookupStrategy(options), qType) {
		return result, nil
	}
	result.addresses = filterAddressesByQueryType(MessageToAddresses(exchangeResult.response), qType)
	return result, nil
}

func (r *Router) Exchange(ctx context.Context, message *mDNS.Msg, options adapter.DNSQueryOptions) (*mDNS.Msg, error) {
	if len(message.Question) != 1 {
		r.logger.WarnContext(ctx, "bad question size: ", len(message.Question))
		responseMessage := mDNS.Msg{
			MsgHdr: mDNS.MsgHdr{
				Id:       message.Id,
				Response: true,
				Rcode:    mDNS.RcodeFormatError,
			},
			Question: message.Question,
		}
		return &responseMessage, nil
	}
	r.rulesAccess.RLock()
	defer r.rulesAccess.RUnlock()
	if r.runtimeRuleError != nil {
		return nil, r.runtimeRuleError
	}
	r.logger.DebugContext(ctx, "exchange ", FormatQuestion(message.Question[0].String()))
	var (
		response  *mDNS.Msg
		transport adapter.DNSTransport
		err       error
	)
	var metadata *adapter.InboundContext
	ctx, metadata = adapter.ExtendContext(ctx)
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
	if options.Transport != nil {
		transport = options.Transport
		if options.Strategy == C.DomainStrategyAsIS {
			options.Strategy = r.defaultDomainStrategy
		}
		response, err = r.client.Exchange(ctx, transport, message, options, nil)
	} else if !r.legacyDNSMode {
		exchangeResult := r.exchangeWithRules(ctx, message, options, true)
		response, transport, err = exchangeResult.response, exchangeResult.transport, exchangeResult.err
	} else {
		var (
			rule      adapter.DNSRule
			ruleIndex int
		)
		ruleIndex = -1
		for {
			dnsCtx := adapter.OverrideContext(ctx)
			dnsOptions := options
			transport, rule, ruleIndex = r.matchDNS(ctx, true, ruleIndex, isAddressQuery(message), &dnsOptions)
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
						}, nil
					case C.RuleActionRejectMethodDrop:
						return nil, tun.ErrDrop
					}
				case *R.RuleActionPredefined:
					return action.Response(message), nil
				}
			}
			responseCheck := addressLimitResponseCheck(rule, metadata)
			if dnsOptions.Strategy == C.DomainStrategyAsIS {
				dnsOptions.Strategy = r.defaultDomainStrategy
			}
			response, err = r.client.Exchange(dnsCtx, transport, message, dnsOptions, responseCheck)
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
			break
		}
	}
	if err != nil {
		return nil, err
	}
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
	return response, nil
}

func (r *Router) Lookup(ctx context.Context, domain string, options adapter.DNSQueryOptions) ([]netip.Addr, error) {
	r.rulesAccess.RLock()
	defer r.rulesAccess.RUnlock()
	if r.runtimeRuleError != nil {
		return nil, r.runtimeRuleError
	}
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
	} else if !r.legacyDNSMode {
		responseAddrs, err = r.lookupWithRules(ctx, domain, options)
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
			transport, rule, ruleIndex = r.matchDNS(ctx, false, ruleIndex, true, &dnsOptions)
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
		checkMetadata.DNSResponse = response
		return rule.MatchAddressLimit(&checkMetadata, response)
	}
}

func (r *Router) ClearCache() {
	r.client.ClearCache()
	if r.platformInterface != nil {
		r.platformInterface.ClearDNSCache()
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
	if rule.IPAcceptAny || rule.RuleSetIPCIDRAcceptEmpty {
		return true
	}
	return !rule.MatchResponse && (len(rule.IPCIDR) > 0 || rule.IPIsPrivate)
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
		rule.IPVersion > 0 ||
		len(rule.QueryType) > 0
}

func resolveLegacyDNSMode(router adapter.Router, rules []option.DNSRule) (bool, error) {
	legacyDNSModeDisabled, needsLegacyDNSMode, needsLegacyDNSModeFromStrategy, err := dnsRuleModeRequirements(router, rules)
	if err != nil {
		return false, err
	}
	if legacyDNSModeDisabled && needsLegacyDNSModeFromStrategy {
		return false, E.New("DNS rule action strategy is only supported in legacyDNSMode")
	}
	if legacyDNSModeDisabled {
		return false, nil
	}
	return needsLegacyDNSMode, nil
}

func dnsRuleModeRequirements(router adapter.Router, rules []option.DNSRule) (bool, bool, bool, error) {
	var legacyDNSModeDisabled bool
	var needsLegacyDNSMode bool
	var needsLegacyDNSModeFromStrategy bool
	for i, rule := range rules {
		ruleLegacyDNSModeDisabled, ruleNeedsLegacyDNSMode, ruleNeedsLegacyDNSModeFromStrategy, err := dnsRuleModeRequirementsInRule(router, rule)
		if err != nil {
			return false, false, false, E.Cause(err, "dns rule[", i, "]")
		}
		legacyDNSModeDisabled = legacyDNSModeDisabled || ruleLegacyDNSModeDisabled
		needsLegacyDNSMode = needsLegacyDNSMode || ruleNeedsLegacyDNSMode
		needsLegacyDNSModeFromStrategy = needsLegacyDNSModeFromStrategy || ruleNeedsLegacyDNSModeFromStrategy
	}
	return legacyDNSModeDisabled, needsLegacyDNSMode, needsLegacyDNSModeFromStrategy, nil
}

func dnsRuleModeRequirementsInRule(router adapter.Router, rule option.DNSRule) (bool, bool, bool, error) {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return dnsRuleModeRequirementsInDefaultRule(router, rule.DefaultOptions)
	case C.RuleTypeLogical:
		legacyDNSModeDisabled := dnsRuleActionType(rule) == C.RuleActionTypeEvaluate
		needsLegacyDNSModeFromStrategy := dnsRuleActionHasStrategy(rule.LogicalOptions.DNSRuleAction)
		needsLegacyDNSMode := needsLegacyDNSModeFromStrategy
		for i, subRule := range rule.LogicalOptions.Rules {
			subLegacyDNSModeDisabled, subNeedsLegacyDNSMode, subNeedsLegacyDNSModeFromStrategy, err := dnsRuleModeRequirementsInRule(router, subRule)
			if err != nil {
				return false, false, false, E.Cause(err, "sub rule[", i, "]")
			}
			legacyDNSModeDisabled = legacyDNSModeDisabled || subLegacyDNSModeDisabled
			needsLegacyDNSMode = needsLegacyDNSMode || subNeedsLegacyDNSMode
			needsLegacyDNSModeFromStrategy = needsLegacyDNSModeFromStrategy || subNeedsLegacyDNSModeFromStrategy
		}
		return legacyDNSModeDisabled, needsLegacyDNSMode, needsLegacyDNSModeFromStrategy, nil
	default:
		return false, false, false, nil
	}
}

func dnsRuleModeRequirementsInDefaultRule(router adapter.Router, rule option.DefaultDNSRule) (bool, bool, bool, error) {
	legacyDNSModeDisabled := defaultRuleDisablesLegacyDNSMode(rule)
	needsLegacyDNSModeFromStrategy := dnsRuleActionHasStrategy(rule.DNSRuleAction)
	needsLegacyDNSMode := defaultRuleNeedsLegacyDNSModeFromAddressFilter(rule) || needsLegacyDNSModeFromStrategy
	if len(rule.RuleSet) == 0 {
		return legacyDNSModeDisabled, needsLegacyDNSMode, needsLegacyDNSModeFromStrategy, nil
	}
	if router == nil {
		return false, false, false, E.New("router service not found")
	}
	for _, tag := range rule.RuleSet {
		ruleSet, loaded := router.RuleSet(tag)
		if !loaded {
			return false, false, false, E.New("rule-set not found: ", tag)
		}
		metadata := ruleSet.Metadata()
		// Rule sets are built from headless rules, so query_type is the only
		// per-query DNS predicate they can contribute here. ip_version is not a
		// headless-rule item and is therefore intentionally absent from metadata.
		legacyDNSModeDisabled = legacyDNSModeDisabled || metadata.ContainsDNSQueryTypeRule
		if !rule.RuleSetIPCIDRMatchSource && metadata.ContainsIPCIDRRule {
			needsLegacyDNSMode = true
		}
	}
	return legacyDNSModeDisabled, needsLegacyDNSMode, needsLegacyDNSModeFromStrategy, nil
}

func referencedDNSRuleSetTags(rules []option.DNSRule) []string {
	tagMap := make(map[string]bool)
	var walkRule func(rule option.DNSRule)
	walkRule = func(rule option.DNSRule) {
		switch rule.Type {
		case "", C.RuleTypeDefault:
			for _, tag := range rule.DefaultOptions.RuleSet {
				tagMap[tag] = true
			}
		case C.RuleTypeLogical:
			for _, subRule := range rule.LogicalOptions.Rules {
				walkRule(subRule)
			}
		}
	}
	for _, rule := range rules {
		walkRule(rule)
	}
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func validateLegacyDNSModeDisabledRules(rules []option.DNSRule) error {
	for i, rule := range rules {
		consumesResponse, err := validateLegacyDNSModeDisabledRuleTree(rule)
		if err != nil {
			return E.Cause(err, "validate dns rule[", i, "]")
		}
		action := dnsRuleActionType(rule)
		if action == C.RuleActionTypeEvaluate && consumesResponse {
			return E.New("dns rule[", i, "]: evaluate rule cannot consume response state")
		}
	}
	return nil
}

func validateLegacyDNSModeDisabledRuleTree(rule option.DNSRule) (bool, error) {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return validateLegacyDNSModeDisabledDefaultRule(rule.DefaultOptions)
	case C.RuleTypeLogical:
		var consumesResponse bool
		for i, subRule := range rule.LogicalOptions.Rules {
			subConsumesResponse, err := validateLegacyDNSModeDisabledRuleTree(subRule)
			if err != nil {
				return false, E.Cause(err, "sub rule[", i, "]")
			}
			consumesResponse = consumesResponse || subConsumesResponse
		}
		return consumesResponse, nil
	default:
		return false, nil
	}
}

func validateLegacyDNSModeDisabledDefaultRule(rule option.DefaultDNSRule) (bool, error) {
	hasResponseRecords := hasResponseMatchFields(rule)
	if hasResponseRecords && !rule.MatchResponse {
		return false, E.New("response_* items require match_response")
	}
	if (len(rule.IPCIDR) > 0 || rule.IPIsPrivate) && !rule.MatchResponse {
		return false, E.New("ip_cidr and ip_is_private require match_response when legacyDNSMode is disabled")
	}
	// Intentionally do not reject rule_set here. A referenced rule set may mix
	// destination-IP predicates with pre-response predicates such as domain items.
	// When match_response is false, those destination-IP branches fail closed during
	// pre-response evaluation instead of consuming DNS response state, while sibling
	// non-response branches remain matchable.
	if rule.IPAcceptAny {
		return false, E.New("ip_accept_any is removed when legacyDNSMode is disabled, use ip_cidr with match_response")
	}
	if rule.RuleSetIPCIDRAcceptEmpty {
		return false, E.New("rule_set_ip_cidr_accept_empty is removed when legacyDNSMode is disabled")
	}
	return rule.MatchResponse, nil
}

func hasDNSRuleActionStrategy(rules []option.DNSRule) bool {
	for _, rule := range rules {
		if dnsRuleHasActionStrategy(rule) {
			return true
		}
	}
	return false
}

func dnsRuleHasActionStrategy(rule option.DNSRule) bool {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		return dnsRuleActionHasStrategy(rule.DefaultOptions.DNSRuleAction)
	case C.RuleTypeLogical:
		if dnsRuleActionHasStrategy(rule.LogicalOptions.DNSRuleAction) {
			return true
		}
		return hasDNSRuleActionStrategy(rule.LogicalOptions.Rules)
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

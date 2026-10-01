package route

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	R "github.com/sagernet/sing-box/route/rule"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/pause"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx               context.Context
	logger            log.ContextLogger
	inbound           adapter.InboundManager
	outbound          adapter.OutboundManager
	dns               adapter.DNSRouter
	dnsTransport      adapter.DNSTransportManager
	connection        adapter.ConnectionManager
	network           adapter.NetworkManager
	httpClientManager adapter.HTTPClientManager
	rules             []adapter.Rule
	needFindProcess   bool
	needFindNeighbor  bool
	leaseFiles        []string
	ruleSets          []adapter.RuleSet
	ruleSetMap        map[string]adapter.RuleSet
	ruleSetUpdater    *R.RuleSetUpdater
	processSearcher   process.Searcher
	processCache      *freelru.Cache[processCacheKey, processCacheEntry]
	neighborResolver  adapter.NeighborResolver
	pauseManager      pause.Manager
	trackers          []adapter.ConnectionTracker
	platformInterface adapter.PlatformInterface
}

func NewRouter(ctx context.Context, logFactory log.Factory, options option.RouteOptions, dnsOptions option.DNSOptions) *Router {
	return &Router{
		ctx:               ctx,
		logger:            logFactory.NewLogger("router"),
		inbound:           service.FromContext[adapter.InboundManager](ctx),
		outbound:          service.FromContext[adapter.OutboundManager](ctx),
		dns:               service.FromContext[adapter.DNSRouter](ctx),
		dnsTransport:      service.FromContext[adapter.DNSTransportManager](ctx),
		connection:        service.FromContext[adapter.ConnectionManager](ctx),
		network:           service.FromContext[adapter.NetworkManager](ctx),
		httpClientManager: service.FromContext[adapter.HTTPClientManager](ctx),
		rules:             make([]adapter.Rule, 0, len(options.Rules)),
		ruleSetMap:        make(map[string]adapter.RuleSet),
		needFindProcess:   hasRule(options.Rules, isProcessRule) || hasDNSRule(dnsOptions.Rules, isProcessDNSRule) || options.FindProcess,
		needFindNeighbor:  hasRule(options.Rules, isNeighborRule) || hasDNSRule(dnsOptions.Rules, isNeighborDNSRule) || hasLocalNeighborDNSServer(dnsOptions.Servers) || options.FindNeighbor,
		leaseFiles:        options.DHCPLeaseFiles,
		pauseManager:      service.FromContext[pause.Manager](ctx),
		platformInterface: service.FromContext[adapter.PlatformInterface](ctx),
	}
}

func (r *Router) Initialize(rules []option.Rule, ruleSets []option.RuleSet) error {
	for i, options := range rules {
		err := R.ValidateNoNestedRuleActions(options)
		if err != nil {
			return E.Cause(err, "parse rule[", i, "]")
		}
		rule, err := R.NewRule(r.ctx, r.logger, options, false)
		if err != nil {
			return E.Cause(err, "parse rule[", i, "]")
		}
		r.rules = append(r.rules, rule)
	}
	for i, options := range ruleSets {
		for _, tag := range options.Tag {
			if _, exists := r.ruleSetMap[tag]; exists {
				return E.New("duplicate rule-set tag: ", tag)
			}
			ruleSet, err := R.NewRuleSet(r.ctx, r.logger, tag, options)
			if err != nil {
				return E.Cause(err, "parse rule-set[", i, "]")
			}
			r.ruleSets = append(r.ruleSets, ruleSet)
			r.ruleSetMap[tag] = ruleSet
		}
	}
	return nil
}

func (r *Router) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	monitor := taskmonitor.New(r.logger, C.StartTimeout)
	switch stage {
	case adapter.StartStateInitialize:
		for _, ruleSet := range r.ruleSets {
			scope.Add(ruleSet.Close)
		}
		if r.needFindNeighbor {
			if r.platformInterface != nil && r.platformInterface.UsePlatformNeighborResolver() {
				monitor.Start("initialize neighbor resolver")
				resolver := newPlatformNeighborResolver(r.logger, r.platformInterface)
				err := resolver.Start()
				monitor.Finish()
				if err != nil {
					r.logger.Error(E.Cause(err, "start neighbor resolver"))
				} else {
					r.neighborResolver = resolver
					scope.Add(resolver.Close)
				}
			} else {
				monitor.Start("initialize neighbor resolver")
				resolver, err := newNeighborResolver(r.logger, r.leaseFiles)
				monitor.Finish()
				if err != nil {
					if err != os.ErrInvalid {
						r.logger.Error(E.Cause(err, "create neighbor resolver"))
					}
				} else {
					err = resolver.Start()
					if err != nil {
						r.logger.Error(E.Cause(err, "start neighbor resolver"))
					} else {
						r.neighborResolver = resolver
						scope.Add(resolver.Close)
					}
				}
			}
		}
	case adapter.StartStateStart:
		var startContext *adapter.HTTPStartContext
		if len(r.ruleSets) > 0 {
			monitor.Start("initialize rule-set")
			startContext = adapter.NewHTTPStartContext()
			var ruleSetStartGroup task.Group
			for i, ruleSet := range r.ruleSets {
				ruleSetInPlace := ruleSet
				ruleSetStartGroup.Append0(func(ctx context.Context) error {
					err := ruleSetInPlace.StartContext(ctx, startContext)
					if err != nil {
						return E.Cause(err, "initialize rule-set[", i, "]")
					}
					return nil
				})
			}
			ruleSetStartGroup.Concurrency(5)
			ruleSetStartGroup.FastFail()
			err := ruleSetStartGroup.Run(r.ctx)
			monitor.Finish()
			if err != nil {
				return err
			}
		}
		if startContext != nil {
			startContext.Close()
		}
		r.ruleSetUpdater = R.NewRuleSetUpdater(r.ctx, r.ruleSets)
		if r.ruleSetUpdater != nil {
			scope.Add(r.ruleSetUpdater.Close)
		}
		r.network.Initialize(r.ruleSets)
		needFindProcess := r.needFindProcess
		for _, ruleSet := range r.ruleSets {
			metadata := ruleSet.Metadata()
			if metadata.ContainsProcessRule {
				needFindProcess = true
			}
		}
		if C.IsAndroid && r.platformInterface != nil {
			needFindProcess = true
		}
		r.needFindProcess = needFindProcess
		if needFindProcess {
			if r.platformInterface != nil && r.platformInterface.UsePlatformConnectionOwnerFinder() {
				r.processSearcher = newPlatformSearcher(r.platformInterface)
			} else {
				monitor.Start("initialize process searcher")
				searcher, err := process.NewSearcher(process.Config{
					Logger:         r.logger,
					PackageManager: r.network.PackageManager(),
				})
				monitor.Finish()
				if err != nil {
					if err != os.ErrInvalid {
						r.logger.Warn(E.Cause(err, "create process searcher"))
					}
				} else {
					r.processSearcher = searcher
				}
			}
		}
		if r.processSearcher != nil {
			scope.Add(r.processSearcher.Close)
			processCache := common.Must1(freelru.New[processCacheKey, processCacheEntry](256, maphash.NewHasher[processCacheKey]().Hash32, true))
			processCache.SetLifetime(200 * time.Millisecond)
			r.processCache = processCache
		}
	case adapter.StartStatePostStart:
		for i, rule := range r.rules {
			scope.Add(rule.Close)
			monitor.Start("initialize rule[", i, "]")
			err := rule.Start()
			monitor.Finish()
			if err != nil {
				return E.Cause(err, "initialize rule[", i, "]")
			}
		}
		if r.ruleSetUpdater != nil {
			r.ruleSetUpdater.Start()
		}
		return nil
	case adapter.StartStateStarted:
		for _, ruleSet := range r.ruleSets {
			ruleSet.Cleanup()
		}
		runtime.GC()
	}
	return nil
}

func (r *Router) RuleSet(tag string) (adapter.RuleSet, bool) {
	ruleSet, loaded := r.ruleSetMap[tag]
	return ruleSet, loaded
}

func (r *Router) Rules() []adapter.Rule {
	return r.rules
}

func (r *Router) AppendTracker(tracker adapter.ConnectionTracker) {
	r.trackers = append(r.trackers, tracker)
}

func (r *Router) NeedFindProcess() bool {
	return r.needFindProcess
}

func (r *Router) NeedFindNeighbor() bool {
	return r.needFindNeighbor
}

func (r *Router) NeighborResolver() adapter.NeighborResolver {
	return r.neighborResolver
}

func (r *Router) ResetNetwork() {
	r.httpClientManager.ResetNetwork()
	r.dns.ResetNetwork()
	if r.processCache != nil {
		r.processCache.Purge()
	}
	if r.processSearcher != nil {
		r.processSearcher.ResetCache()
	}
}

package route

import (
	"context"
	"path/filepath"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
)

var _ adapter.RuleSet = (*LocalRuleSet)(nil)

type LocalRuleSet struct {
	abstractRuleSet
	watcher *fswatch.Watcher
}

func NewLocalRuleSet(ctx context.Context, router adapter.Router, logger logger.ContextLogger, options option.RuleSet) (*LocalRuleSet, error) {
	ruleSet := &LocalRuleSet{
		abstractRuleSet: abstractRuleSet{
			router: router,
			logger: logger,
			tag:    options.Tag,
		},
	}
	if options.Type == C.RuleSetTypeInline {
		if len(options.InlineOptions.Rules) == 0 {
			return nil, E.New("empty inline rule-set")
		}
		err := ruleSet.reloadRules(options.InlineOptions.Rules)
		if err != nil {
			return nil, err
		}
		return ruleSet, nil
	}
	ruleSet.path = options.Path
	ruleSet.format = options.Format
	path, err := ruleSet.getPath(options.Path)
	if err != nil {
		return nil, err
	}
	err = ruleSet.loadFromFile(path)
	if err != nil {
		return nil, err
	}
	var watcher *fswatch.Watcher
	filePath, _ := filepath.Abs(path)
	watcher, err = fswatch.NewWatcher(fswatch.Options{
		Path: []string{filePath},
		Callback: func(path string) {
			uErr := ruleSet.loadFromFile(path)
			if uErr != nil {
				logger.ErrorContext(log.ContextWithNewID(context.Background()), E.Cause(uErr, "reload rule-set ", options.Tag))
			}
		},
	})
	if err != nil {
		return nil, err
	}
	ruleSet.watcher = watcher
	return ruleSet, nil
}

func (s *LocalRuleSet) StartContext(ctx context.Context, startContext adapter.RuleSetStartContext) error {
	if s.watcher != nil {
		err := s.watcher.Start()
		if err != nil {
			s.logger.Error(E.Cause(err, "watch rule-set file"))
		}
	}
	return nil
}

func (s *LocalRuleSet) PostStart() error {
	return nil
}

func (s *LocalRuleSet) RegisterCallback(callback adapter.RuleSetUpdateCallback) *list.Element[adapter.RuleSetUpdateCallback] {
	return nil
}

func (s *LocalRuleSet) UnregisterCallback(element *list.Element[adapter.RuleSetUpdateCallback]) {
}

func (s *LocalRuleSet) Close() error {
	s.rules = nil
	return common.Close(common.PtrOrNil(s.watcher))
}

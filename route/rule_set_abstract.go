package route

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/atomic"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/rw"

	"go4.org/netipx"
)

type abstractRuleSet struct {
	router      adapter.Router
	logger      logger.ContextLogger
	tag         string
	path        string
	format      string
	rules       []adapter.HeadlessRule
	metadata    adapter.RuleSetMetadata
	lastUpdated time.Time
	refs        atomic.Int32
}

func (s *abstractRuleSet) Name() string {
	return s.tag
}

func (s *abstractRuleSet) String() string {
	return strings.Join(F.MapToString(s.rules), " ")
}

func (s *abstractRuleSet) getPath(path string) (string, error) {
	if path == "" {
		path = s.tag
		switch s.format {
		case C.RuleSetFormatSource, "":
			path += ".json"
		case C.RuleSetFormatBinary:
			path += ".srs"
		}
	}
	if rw.IsDir(path) {
		return "", E.New("rule_set path is a directory: ", path)
	}
	return path, nil
}

func (s *abstractRuleSet) Metadata() adapter.RuleSetMetadata {
	return s.metadata
}

func (s *abstractRuleSet) ExtractIPSet() []*netipx.IPSet {
	return common.FlatMap(s.rules, extractIPSetFromRule)
}

func (s *abstractRuleSet) IncRef() {
	s.refs.Add(1)
}

func (s *abstractRuleSet) DecRef() {
	if s.refs.Add(-1) < 0 {
		panic("rule-set: negative refs")
	}
}

func (s *abstractRuleSet) Cleanup() {
	if s.refs.Load() == 0 {
		s.rules = nil
	}
}

func (s *abstractRuleSet) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = s.loadBytes(content)
	if err != nil {
		return err
	}
	fs, _ := file.Stat()
	s.lastUpdated = fs.ModTime()
	return nil
}

func (s *abstractRuleSet) loadBytes(content []byte) error {
	var (
		plainRuleSet option.PlainRuleSet
		err          error
	)
	switch s.format {
	case C.RuleSetFormatSource:
		var compat option.PlainRuleSetCompat
		compat, err = json.UnmarshalExtended[option.PlainRuleSetCompat](content)
		if err != nil {
			return err
		}
		plainRuleSet, err = compat.Upgrade()
		if err != nil {
			return err
		}
	case C.RuleSetFormatBinary:
		plainRuleSet, err = srs.Read(bytes.NewReader(content), false)
		if err != nil {
			return err
		}
	default:
		return E.New("unknown rule-set format: ", s.format)
	}
	return s.reloadRules(plainRuleSet.Rules)
}

func (s *abstractRuleSet) reloadRules(headlessRules []option.HeadlessRule) error {
	rules := make([]adapter.HeadlessRule, len(headlessRules))
	var err error
	for i, ruleOptions := range headlessRules {
		rules[i], err = NewHeadlessRule(s.router, ruleOptions)
		if err != nil {
			return E.Cause(err, "parse rule_set.rules.[", i, "]")
		}
	}
	var metadata adapter.RuleSetMetadata
	metadata.ContainsProcessRule = hasHeadlessRule(headlessRules, isProcessHeadlessRule)
	metadata.ContainsWIFIRule = hasHeadlessRule(headlessRules, isWIFIHeadlessRule)
	metadata.ContainsIPCIDRRule = hasHeadlessRule(headlessRules, isIPCIDRHeadlessRule)
	s.rules = rules
	s.metadata = metadata
	return nil
}

func (s *abstractRuleSet) Match(metadata *adapter.InboundContext) bool {
	for _, rule := range s.rules {
		if rule.Match(metadata) {
			return true
		}
	}
	return false
}

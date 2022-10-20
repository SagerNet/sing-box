package balancer

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

type weightScaler func(value, weight float64) float64

var numberFinder = regexp.MustCompile(`\d+(\.\d+)?`)

// NewWeightManager creates a new WeightManager with settings
func NewWeightManager(logger log.Logger, s []*option.StrategyWeight, defaultWeight float64, scaler weightScaler) *WeightManager {
	return &WeightManager{
		settings:      s,
		cache:         make(map[string]float64),
		scaler:        scaler,
		defaultWeight: defaultWeight,
		logger:        logger,
	}
}

// WeightManager manages weights for specific settings
type WeightManager struct {
	settings      []*option.StrategyWeight
	cache         map[string]float64
	scaler        weightScaler
	defaultWeight float64
	logger        log.Logger
}

// Get get the weight of specified tag
func (s *WeightManager) Get(tag string) float64 {
	weight, ok := s.cache[tag]
	if ok {
		return weight
	}
	weight = s.findValue(tag)
	s.cache[tag] = weight
	return weight
}

// Apply applies weight to the value
func (s *WeightManager) Apply(tag string, value float64) float64 {
	return s.scaler(value, s.Get(tag))
}

func (s *WeightManager) findValue(tag string) float64 {
	for _, w := range s.settings {
		matched := s.getMatch(tag, w.Match, w.Regexp)
		if matched == "" {
			continue
		}
		if w.Value > 0 {
			return float64(w.Value)
		}
		// auto weight from matched
		numStr := numberFinder.FindString(matched)
		if numStr == "" {
			return s.defaultWeight
		}
		weight, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			s.logger.Warn(E.Cause(err, "parse weight from tag"))
			return s.defaultWeight
		}
		return weight
	}
	return s.defaultWeight
}

func (s *WeightManager) getMatch(tag, find string, isRegexp bool) string {
	if !isRegexp {
		idx := strings.Index(tag, find)
		if idx < 0 {
			return ""
		}
		return find
	}
	r, err := regexp.Compile(find)
	if err != nil {
		s.logger.Warn(E.Cause(err, "weight regexp"))
		return ""
	}
	return r.FindString(tag)
}

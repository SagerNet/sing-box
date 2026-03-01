package ocm

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

type UsageStats struct {
	RequestCount int   `json:"request_count"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
}

func (u *UsageStats) UnmarshalJSON(data []byte) error {
	type Alias UsageStats
	aux := &struct {
		*Alias
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	}{
		Alias: (*Alias)(u),
	}
	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}
	if u.InputTokens == 0 && aux.PromptTokens > 0 {
		u.InputTokens = aux.PromptTokens
	}
	if u.OutputTokens == 0 && aux.CompletionTokens > 0 {
		u.OutputTokens = aux.CompletionTokens
	}
	return nil
}

type CostCombination struct {
	Model         string                `json:"model"`
	ServiceTier   string                `json:"service_tier,omitempty"`
	WeekStartUnix int64                 `json:"week_start_unix,omitempty"`
	Total         UsageStats            `json:"total"`
	ByUser        map[string]UsageStats `json:"by_user"`
}

type AggregatedUsage struct {
	LastUpdated  time.Time         `json:"last_updated"`
	Combinations []CostCombination `json:"combinations"`
	mutex        sync.Mutex
	filePath     string
	logger       log.ContextLogger
	lastSaveTime time.Time
	pendingSave  bool
	saveTimer    *time.Timer
	saveMutex    sync.Mutex
}

type UsageStatsJSON struct {
	RequestCount int     `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type CostCombinationJSON struct {
	Model         string                    `json:"model"`
	ServiceTier   string                    `json:"service_tier,omitempty"`
	WeekStartUnix int64                     `json:"week_start_unix,omitempty"`
	Total         UsageStatsJSON            `json:"total"`
	ByUser        map[string]UsageStatsJSON `json:"by_user"`
}

type CostsSummaryJSON struct {
	TotalUSD float64            `json:"total_usd"`
	ByUser   map[string]float64 `json:"by_user"`
	ByWeek   map[string]float64 `json:"by_week,omitempty"`
}

type AggregatedUsageJSON struct {
	LastUpdated  time.Time             `json:"last_updated"`
	Costs        CostsSummaryJSON      `json:"costs"`
	Combinations []CostCombinationJSON `json:"combinations"`
}

type WeeklyCycleHint struct {
	WindowMinutes int64
	ResetAt       time.Time
}

type ModelPricing struct {
	InputPrice       float64
	OutputPrice      float64
	CachedInputPrice float64
}

type modelFamily struct {
	pattern *regexp.Regexp
	pricing ModelPricing
}

const (
	serviceTierAuto     = "auto"
	serviceTierDefault  = "default"
	serviceTierFlex     = "flex"
	serviceTierPriority = "priority"
	serviceTierScale    = "scale"
)

var (
	gpt52Pricing = ModelPricing{
		InputPrice:       1.75,
		OutputPrice:      14.0,
		CachedInputPrice: 0.175,
	}

	gpt5Pricing = ModelPricing{
		InputPrice:       1.25,
		OutputPrice:      10.0,
		CachedInputPrice: 0.125,
	}

	gpt5MiniPricing = ModelPricing{
		InputPrice:       0.25,
		OutputPrice:      2.0,
		CachedInputPrice: 0.025,
	}

	gpt5NanoPricing = ModelPricing{
		InputPrice:       0.05,
		OutputPrice:      0.4,
		CachedInputPrice: 0.005,
	}

	gpt52CodexPricing = ModelPricing{
		InputPrice:       1.75,
		OutputPrice:      14.0,
		CachedInputPrice: 0.175,
	}

	gpt51CodexPricing = ModelPricing{
		InputPrice:       1.25,
		OutputPrice:      10.0,
		CachedInputPrice: 0.125,
	}

	gpt51CodexMiniPricing = ModelPricing{
		InputPrice:       0.25,
		OutputPrice:      2.0,
		CachedInputPrice: 0.025,
	}

	gpt52ProPricing = ModelPricing{
		InputPrice:       21.0,
		OutputPrice:      168.0,
		CachedInputPrice: 21.0,
	}

	gpt5ProPricing = ModelPricing{
		InputPrice:       15.0,
		OutputPrice:      120.0,
		CachedInputPrice: 15.0,
	}

	gpt52FlexPricing = ModelPricing{
		InputPrice:       0.875,
		OutputPrice:      7.0,
		CachedInputPrice: 0.0875,
	}

	gpt5FlexPricing = ModelPricing{
		InputPrice:       0.625,
		OutputPrice:      5.0,
		CachedInputPrice: 0.0625,
	}

	gpt5MiniFlexPricing = ModelPricing{
		InputPrice:       0.125,
		OutputPrice:      1.0,
		CachedInputPrice: 0.0125,
	}

	gpt5NanoFlexPricing = ModelPricing{
		InputPrice:       0.025,
		OutputPrice:      0.2,
		CachedInputPrice: 0.0025,
	}

	gpt52PriorityPricing = ModelPricing{
		InputPrice:       3.5,
		OutputPrice:      28.0,
		CachedInputPrice: 0.35,
	}

	gpt5PriorityPricing = ModelPricing{
		InputPrice:       2.5,
		OutputPrice:      20.0,
		CachedInputPrice: 0.25,
	}

	gpt5MiniPriorityPricing = ModelPricing{
		InputPrice:       0.45,
		OutputPrice:      3.6,
		CachedInputPrice: 0.045,
	}

	gpt52CodexPriorityPricing = ModelPricing{
		InputPrice:       3.5,
		OutputPrice:      28.0,
		CachedInputPrice: 0.35,
	}

	gpt51CodexPriorityPricing = ModelPricing{
		InputPrice:       2.5,
		OutputPrice:      20.0,
		CachedInputPrice: 0.25,
	}

	gpt4oPricing = ModelPricing{
		InputPrice:       2.5,
		OutputPrice:      10.0,
		CachedInputPrice: 1.25,
	}

	gpt4oMiniPricing = ModelPricing{
		InputPrice:       0.15,
		OutputPrice:      0.6,
		CachedInputPrice: 0.075,
	}

	gpt4oAudioPricing = ModelPricing{
		InputPrice:       2.5,
		OutputPrice:      10.0,
		CachedInputPrice: 2.5,
	}

	gpt4oMiniAudioPricing = ModelPricing{
		InputPrice:       0.15,
		OutputPrice:      0.6,
		CachedInputPrice: 0.15,
	}

	gptAudioMiniPricing = ModelPricing{
		InputPrice:       0.6,
		OutputPrice:      2.4,
		CachedInputPrice: 0.6,
	}

	o1Pricing = ModelPricing{
		InputPrice:       15.0,
		OutputPrice:      60.0,
		CachedInputPrice: 7.5,
	}

	o1ProPricing = ModelPricing{
		InputPrice:       150.0,
		OutputPrice:      600.0,
		CachedInputPrice: 150.0,
	}

	o1MiniPricing = ModelPricing{
		InputPrice:       1.1,
		OutputPrice:      4.4,
		CachedInputPrice: 0.55,
	}

	o3MiniPricing = ModelPricing{
		InputPrice:       1.1,
		OutputPrice:      4.4,
		CachedInputPrice: 0.55,
	}

	o3Pricing = ModelPricing{
		InputPrice:       2.0,
		OutputPrice:      8.0,
		CachedInputPrice: 0.5,
	}

	o3ProPricing = ModelPricing{
		InputPrice:       20.0,
		OutputPrice:      80.0,
		CachedInputPrice: 20.0,
	}

	o3DeepResearchPricing = ModelPricing{
		InputPrice:       10.0,
		OutputPrice:      40.0,
		CachedInputPrice: 2.5,
	}

	o4MiniPricing = ModelPricing{
		InputPrice:       1.1,
		OutputPrice:      4.4,
		CachedInputPrice: 0.275,
	}

	o4MiniDeepResearchPricing = ModelPricing{
		InputPrice:       2.0,
		OutputPrice:      8.0,
		CachedInputPrice: 0.5,
	}

	o3FlexPricing = ModelPricing{
		InputPrice:       1.0,
		OutputPrice:      4.0,
		CachedInputPrice: 0.25,
	}

	o4MiniFlexPricing = ModelPricing{
		InputPrice:       0.55,
		OutputPrice:      2.2,
		CachedInputPrice: 0.138,
	}

	o3PriorityPricing = ModelPricing{
		InputPrice:       3.5,
		OutputPrice:      14.0,
		CachedInputPrice: 0.875,
	}

	o4MiniPriorityPricing = ModelPricing{
		InputPrice:       2.0,
		OutputPrice:      8.0,
		CachedInputPrice: 0.5,
	}

	gpt41Pricing = ModelPricing{
		InputPrice:       2.0,
		OutputPrice:      8.0,
		CachedInputPrice: 0.5,
	}

	gpt41MiniPricing = ModelPricing{
		InputPrice:       0.4,
		OutputPrice:      1.6,
		CachedInputPrice: 0.1,
	}

	gpt41NanoPricing = ModelPricing{
		InputPrice:       0.1,
		OutputPrice:      0.4,
		CachedInputPrice: 0.025,
	}

	gpt41PriorityPricing = ModelPricing{
		InputPrice:       3.5,
		OutputPrice:      14.0,
		CachedInputPrice: 0.875,
	}

	gpt41MiniPriorityPricing = ModelPricing{
		InputPrice:       0.7,
		OutputPrice:      2.8,
		CachedInputPrice: 0.175,
	}

	gpt41NanoPriorityPricing = ModelPricing{
		InputPrice:       0.2,
		OutputPrice:      0.8,
		CachedInputPrice: 0.05,
	}

	gpt4oPriorityPricing = ModelPricing{
		InputPrice:       4.25,
		OutputPrice:      17.0,
		CachedInputPrice: 2.125,
	}

	gpt4oMiniPriorityPricing = ModelPricing{
		InputPrice:       0.25,
		OutputPrice:      1.0,
		CachedInputPrice: 0.125,
	}

	standardModelFamilies = []modelFamily{
		{
			pattern: regexp.MustCompile(`^gpt-5\.3-codex(?:$|-)`),
			pricing: gpt52CodexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2-codex(?:$|-)`),
			pricing: gpt52CodexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-codex-max(?:$|-)`),
			pricing: gpt51CodexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-codex-mini(?:$|-)`),
			pricing: gpt51CodexMiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-codex(?:$|-)`),
			pricing: gpt51CodexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-codex-mini(?:$|-)`),
			pricing: gpt51CodexMiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-codex(?:$|-)`),
			pricing: gpt51CodexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2-chat-latest$`),
			pricing: gpt52Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-chat-latest$`),
			pricing: gpt5Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-chat-latest$`),
			pricing: gpt5Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2-pro(?:$|-)`),
			pricing: gpt52ProPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-pro(?:$|-)`),
			pricing: gpt5ProPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-mini(?:$|-)`),
			pricing: gpt5MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-nano(?:$|-)`),
			pricing: gpt5NanoPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2(?:$|-)`),
			pricing: gpt52Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1(?:$|-)`),
			pricing: gpt5Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5(?:$|-)`),
			pricing: gpt5Pricing,
		},
		{
			pattern: regexp.MustCompile(`^o4-mini-deep-research(?:$|-)`),
			pricing: o4MiniDeepResearchPricing,
		},
		{
			pattern: regexp.MustCompile(`^o4-mini(?:$|-)`),
			pricing: o4MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3-pro(?:$|-)`),
			pricing: o3ProPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3-deep-research(?:$|-)`),
			pricing: o3DeepResearchPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3-mini(?:$|-)`),
			pricing: o3MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3(?:$|-)`),
			pricing: o3Pricing,
		},
		{
			pattern: regexp.MustCompile(`^o1-pro(?:$|-)`),
			pricing: o1ProPricing,
		},
		{
			pattern: regexp.MustCompile(`^o1-mini(?:$|-)`),
			pricing: o1MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o1(?:$|-)`),
			pricing: o1Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o-mini-audio(?:$|-)`),
			pricing: gpt4oMiniAudioPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-audio-mini(?:$|-)`),
			pricing: gptAudioMiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^(?:gpt-4o-audio|gpt-audio)(?:$|-)`),
			pricing: gpt4oAudioPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-nano(?:$|-)`),
			pricing: gpt41NanoPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-mini(?:$|-)`),
			pricing: gpt41MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1(?:$|-)`),
			pricing: gpt41Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o-mini(?:$|-)`),
			pricing: gpt4oMiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o(?:$|-)`),
			pricing: gpt4oPricing,
		},
		{
			pattern: regexp.MustCompile(`^chatgpt-4o(?:$|-)`),
			pricing: gpt4oPricing,
		},
	}

	flexModelFamilies = []modelFamily{
		{
			pattern: regexp.MustCompile(`^gpt-5-mini(?:$|-)`),
			pricing: gpt5MiniFlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-nano(?:$|-)`),
			pricing: gpt5NanoFlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2(?:$|-)`),
			pricing: gpt52FlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1(?:$|-)`),
			pricing: gpt5FlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5(?:$|-)`),
			pricing: gpt5FlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^o4-mini(?:$|-)`),
			pricing: o4MiniFlexPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3(?:$|-)`),
			pricing: o3FlexPricing,
		},
	}

	priorityModelFamilies = []modelFamily{
		{
			pattern: regexp.MustCompile(`^gpt-5\.3-codex(?:$|-)`),
			pricing: gpt52CodexPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2-codex(?:$|-)`),
			pricing: gpt52CodexPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-codex-max(?:$|-)`),
			pricing: gpt51CodexPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1-codex(?:$|-)`),
			pricing: gpt51CodexPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-codex-mini(?:$|-)`),
			pricing: gpt5MiniPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-codex(?:$|-)`),
			pricing: gpt51CodexPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5-mini(?:$|-)`),
			pricing: gpt5MiniPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.2(?:$|-)`),
			pricing: gpt52PriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5\.1(?:$|-)`),
			pricing: gpt5PriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-5(?:$|-)`),
			pricing: gpt5PriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^o4-mini(?:$|-)`),
			pricing: o4MiniPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3(?:$|-)`),
			pricing: o3PriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-nano(?:$|-)`),
			pricing: gpt41NanoPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-mini(?:$|-)`),
			pricing: gpt41MiniPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1(?:$|-)`),
			pricing: gpt41PriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o-mini(?:$|-)`),
			pricing: gpt4oMiniPriorityPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o(?:$|-)`),
			pricing: gpt4oPriorityPricing,
		},
	}
)

func modelFamiliesForTier(serviceTier string) []modelFamily {
	switch serviceTier {
	case serviceTierFlex:
		return flexModelFamilies
	case serviceTierPriority:
		return priorityModelFamilies
	default:
		return standardModelFamilies
	}
}

func findPricingInFamilies(model string, modelFamilies []modelFamily) (ModelPricing, bool) {
	for _, family := range modelFamilies {
		if family.pattern.MatchString(model) {
			return family.pricing, true
		}
	}
	return ModelPricing{}, false
}

func normalizeServiceTier(serviceTier string) string {
	switch strings.ToLower(strings.TrimSpace(serviceTier)) {
	case "", serviceTierAuto, serviceTierDefault:
		return serviceTierDefault
	case serviceTierFlex:
		return serviceTierFlex
	case serviceTierPriority:
		return serviceTierPriority
	case serviceTierScale:
		// Scale-tier requests are prepaid differently and not listed in this usage file.
		return serviceTierDefault
	default:
		return serviceTierDefault
	}
}

func getPricing(model string, serviceTier string) ModelPricing {
	normalizedServiceTier := normalizeServiceTier(serviceTier)
	modelFamilies := modelFamiliesForTier(normalizedServiceTier)

	if pricing, found := findPricingInFamilies(model, modelFamilies); found {
		return pricing
	}

	normalizedModel := normalizeGPT5Model(model)
	if normalizedModel != model {
		if pricing, found := findPricingInFamilies(normalizedModel, modelFamilies); found {
			return pricing
		}
	}

	if normalizedServiceTier != serviceTierDefault {
		if pricing, found := findPricingInFamilies(model, standardModelFamilies); found {
			return pricing
		}
		if normalizedModel != model {
			if pricing, found := findPricingInFamilies(normalizedModel, standardModelFamilies); found {
				return pricing
			}
		}
	}

	return gpt4oPricing
}

func normalizeGPT5Model(model string) string {
	if !strings.HasPrefix(model, "gpt-5.") {
		return model
	}

	switch {
	case strings.Contains(model, "-codex-mini"):
		return "gpt-5.1-codex-mini"
	case strings.Contains(model, "-codex-max"):
		return "gpt-5.1-codex-max"
	case strings.Contains(model, "-codex"):
		return "gpt-5.3-codex"
	case strings.Contains(model, "-chat-latest"):
		return "gpt-5.2-chat-latest"
	case strings.Contains(model, "-pro"):
		return "gpt-5.2-pro"
	case strings.Contains(model, "-mini"):
		return "gpt-5-mini"
	case strings.Contains(model, "-nano"):
		return "gpt-5-nano"
	default:
		return "gpt-5.2"
	}
}

func calculateCost(stats UsageStats, model string, serviceTier string) float64 {
	pricing := getPricing(model, serviceTier)

	regularInputTokens := stats.InputTokens - stats.CachedTokens
	if regularInputTokens < 0 {
		regularInputTokens = 0
	}

	cost := (float64(regularInputTokens)*pricing.InputPrice +
		float64(stats.OutputTokens)*pricing.OutputPrice +
		float64(stats.CachedTokens)*pricing.CachedInputPrice) / 1_000_000

	return math.Round(cost*100) / 100
}

func roundCost(cost float64) float64 {
	return math.Round(cost*100) / 100
}

func normalizeCombinations(combinations []CostCombination) {
	for index := range combinations {
		combinations[index].ServiceTier = normalizeServiceTier(combinations[index].ServiceTier)
		if combinations[index].ByUser == nil {
			combinations[index].ByUser = make(map[string]UsageStats)
		}
	}
}

func addUsageToCombinations(combinations *[]CostCombination, model string, serviceTier string, weekStartUnix int64, user string, inputTokens, outputTokens, cachedTokens int64) {
	var matchedCombination *CostCombination
	for index := range *combinations {
		combination := &(*combinations)[index]
		combinationServiceTier := normalizeServiceTier(combination.ServiceTier)
		if combination.ServiceTier != combinationServiceTier {
			combination.ServiceTier = combinationServiceTier
		}
		if combination.Model == model && combinationServiceTier == serviceTier && combination.WeekStartUnix == weekStartUnix {
			matchedCombination = combination
			break
		}
	}

	if matchedCombination == nil {
		newCombination := CostCombination{
			Model:         model,
			ServiceTier:   serviceTier,
			WeekStartUnix: weekStartUnix,
			Total:         UsageStats{},
			ByUser:        make(map[string]UsageStats),
		}
		*combinations = append(*combinations, newCombination)
		matchedCombination = &(*combinations)[len(*combinations)-1]
	}

	matchedCombination.Total.RequestCount++
	matchedCombination.Total.InputTokens += inputTokens
	matchedCombination.Total.OutputTokens += outputTokens
	matchedCombination.Total.CachedTokens += cachedTokens

	if user != "" {
		userStats := matchedCombination.ByUser[user]
		userStats.RequestCount++
		userStats.InputTokens += inputTokens
		userStats.OutputTokens += outputTokens
		userStats.CachedTokens += cachedTokens
		matchedCombination.ByUser[user] = userStats
	}
}

func buildCombinationJSON(combinations []CostCombination, aggregateUserCosts map[string]float64) ([]CostCombinationJSON, float64) {
	result := make([]CostCombinationJSON, len(combinations))
	var totalCost float64

	for index, combination := range combinations {
		combinationTotalCost := calculateCost(combination.Total, combination.Model, combination.ServiceTier)
		totalCost += combinationTotalCost

		combinationJSON := CostCombinationJSON{
			Model:         combination.Model,
			ServiceTier:   combination.ServiceTier,
			WeekStartUnix: combination.WeekStartUnix,
			Total: UsageStatsJSON{
				RequestCount: combination.Total.RequestCount,
				InputTokens:  combination.Total.InputTokens,
				OutputTokens: combination.Total.OutputTokens,
				CachedTokens: combination.Total.CachedTokens,
				CostUSD:      combinationTotalCost,
			},
			ByUser: make(map[string]UsageStatsJSON),
		}

		for user, userStats := range combination.ByUser {
			userCost := calculateCost(userStats, combination.Model, combination.ServiceTier)
			if aggregateUserCosts != nil {
				aggregateUserCosts[user] += userCost
			}

			combinationJSON.ByUser[user] = UsageStatsJSON{
				RequestCount: userStats.RequestCount,
				InputTokens:  userStats.InputTokens,
				OutputTokens: userStats.OutputTokens,
				CachedTokens: userStats.CachedTokens,
				CostUSD:      userCost,
			}
		}

		result[index] = combinationJSON
	}

	return result, roundCost(totalCost)
}

func formatUTCOffsetLabel(timestamp time.Time) string {
	_, offsetSeconds := timestamp.Zone()
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	offsetHours := offsetSeconds / 3600
	offsetMinutes := (offsetSeconds % 3600) / 60
	if offsetMinutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, offsetHours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, offsetHours, offsetMinutes)
}

func formatWeekStartKey(cycleStartAt time.Time) string {
	localCycleStart := cycleStartAt.In(time.Local)
	return fmt.Sprintf("%s %s", localCycleStart.Format("2006-01-02 15:04:05"), formatUTCOffsetLabel(localCycleStart))
}

func buildByWeekCost(combinations []CostCombination) map[string]float64 {
	byWeek := make(map[string]float64)
	for _, combination := range combinations {
		if combination.WeekStartUnix <= 0 {
			continue
		}
		weekStartAt := time.Unix(combination.WeekStartUnix, 0).UTC()
		weekKey := formatWeekStartKey(weekStartAt)
		byWeek[weekKey] += calculateCost(combination.Total, combination.Model, combination.ServiceTier)
	}
	for weekKey, weekCost := range byWeek {
		byWeek[weekKey] = roundCost(weekCost)
	}
	return byWeek
}

func deriveWeekStartUnix(cycleHint *WeeklyCycleHint) int64 {
	if cycleHint == nil || cycleHint.WindowMinutes <= 0 || cycleHint.ResetAt.IsZero() {
		return 0
	}
	windowDuration := time.Duration(cycleHint.WindowMinutes) * time.Minute
	return cycleHint.ResetAt.UTC().Add(-windowDuration).Unix()
}

func (u *AggregatedUsage) ToJSON() *AggregatedUsageJSON {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	result := &AggregatedUsageJSON{
		LastUpdated: u.LastUpdated,
		Costs: CostsSummaryJSON{
			TotalUSD: 0,
			ByUser:   make(map[string]float64),
			ByWeek:   make(map[string]float64),
		},
	}

	globalCombinationsJSON, totalCost := buildCombinationJSON(u.Combinations, result.Costs.ByUser)
	result.Combinations = globalCombinationsJSON
	result.Costs.TotalUSD = totalCost
	result.Costs.ByWeek = buildByWeekCost(u.Combinations)

	if len(result.Costs.ByWeek) == 0 {
		result.Costs.ByWeek = nil
	}

	for user, cost := range result.Costs.ByUser {
		result.Costs.ByUser[user] = roundCost(cost)
	}

	return result
}

func (u *AggregatedUsage) Load() error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.LastUpdated = time.Time{}
	u.Combinations = nil

	data, err := os.ReadFile(u.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var temp struct {
		LastUpdated  time.Time         `json:"last_updated"`
		Combinations []CostCombination `json:"combinations"`
	}

	err = json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	u.LastUpdated = temp.LastUpdated
	u.Combinations = temp.Combinations
	normalizeCombinations(u.Combinations)

	return nil
}

func (u *AggregatedUsage) Save() error {
	jsonData := u.ToJSON()

	data, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := u.filePath + ".tmp"
	err = os.WriteFile(tmpFile, data, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)
	err = os.Rename(tmpFile, u.filePath)
	if err == nil {
		u.saveMutex.Lock()
		u.lastSaveTime = time.Now()
		u.saveMutex.Unlock()
	}
	return err
}

func (u *AggregatedUsage) AddUsage(model string, inputTokens, outputTokens, cachedTokens int64, serviceTier string, user string) error {
	return u.AddUsageWithCycleHint(model, inputTokens, outputTokens, cachedTokens, serviceTier, user, time.Now(), nil)
}

func (u *AggregatedUsage) AddUsageWithCycleHint(model string, inputTokens, outputTokens, cachedTokens int64, serviceTier string, user string, observedAt time.Time, cycleHint *WeeklyCycleHint) error {
	if model == "" {
		return E.New("model cannot be empty")
	}

	normalizedServiceTier := normalizeServiceTier(serviceTier)
	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.LastUpdated = observedAt
	weekStartUnix := deriveWeekStartUnix(cycleHint)

	addUsageToCombinations(&u.Combinations, model, normalizedServiceTier, weekStartUnix, user, inputTokens, outputTokens, cachedTokens)

	go u.scheduleSave()

	return nil
}

func (u *AggregatedUsage) scheduleSave() {
	const saveInterval = time.Minute

	u.saveMutex.Lock()
	defer u.saveMutex.Unlock()

	timeSinceLastSave := time.Since(u.lastSaveTime)

	if timeSinceLastSave >= saveInterval {
		go u.saveAsync()
		return
	}

	if u.pendingSave {
		return
	}

	u.pendingSave = true
	remainingTime := saveInterval - timeSinceLastSave

	u.saveTimer = time.AfterFunc(remainingTime, func() {
		u.saveMutex.Lock()
		u.pendingSave = false
		u.saveMutex.Unlock()
		u.saveAsync()
	})
}

func (u *AggregatedUsage) saveAsync() {
	err := u.Save()
	if err != nil {
		if u.logger != nil {
			u.logger.Error("save usage statistics: ", err)
		}
	}
}

func (u *AggregatedUsage) cancelPendingSave() {
	u.saveMutex.Lock()
	defer u.saveMutex.Unlock()

	if u.saveTimer != nil {
		u.saveTimer.Stop()
		u.saveTimer = nil
	}
	u.pendingSave = false
}

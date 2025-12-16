package ocm

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
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
	Model  string                `json:"model"`
	Total  UsageStats            `json:"total"`
	ByUser map[string]UsageStats `json:"by_user"`
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
	Model  string                    `json:"model"`
	Total  UsageStatsJSON            `json:"total"`
	ByUser map[string]UsageStatsJSON `json:"by_user"`
}

type CostsSummaryJSON struct {
	TotalUSD float64            `json:"total_usd"`
	ByUser   map[string]float64 `json:"by_user"`
}

type AggregatedUsageJSON struct {
	LastUpdated  time.Time             `json:"last_updated"`
	Costs        CostsSummaryJSON      `json:"costs"`
	Combinations []CostCombinationJSON `json:"combinations"`
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

var (
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
		CachedInputPrice: 1.25,
	}

	o1Pricing = ModelPricing{
		InputPrice:       15.0,
		OutputPrice:      60.0,
		CachedInputPrice: 7.5,
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
		CachedInputPrice: 1.0,
	}

	o4MiniPricing = ModelPricing{
		InputPrice:       1.1,
		OutputPrice:      4.4,
		CachedInputPrice: 0.55,
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

	modelFamilies = []modelFamily{
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-nano`),
			pricing: gpt41NanoPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1-mini`),
			pricing: gpt41MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4\.1`),
			pricing: gpt41Pricing,
		},
		{
			pattern: regexp.MustCompile(`^o4-mini`),
			pricing: o4MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3-mini`),
			pricing: o3MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o3`),
			pricing: o3Pricing,
		},
		{
			pattern: regexp.MustCompile(`^o1-mini`),
			pricing: o1MiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^o1`),
			pricing: o1Pricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o-audio`),
			pricing: gpt4oAudioPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o-mini`),
			pricing: gpt4oMiniPricing,
		},
		{
			pattern: regexp.MustCompile(`^gpt-4o`),
			pricing: gpt4oPricing,
		},
		{
			pattern: regexp.MustCompile(`^chatgpt-4o`),
			pricing: gpt4oPricing,
		},
	}
)

func getPricing(model string) ModelPricing {
	for _, family := range modelFamilies {
		if family.pattern.MatchString(model) {
			return family.pricing
		}
	}
	return gpt4oPricing
}

func calculateCost(stats UsageStats, model string) float64 {
	pricing := getPricing(model)

	regularInputTokens := stats.InputTokens - stats.CachedTokens
	if regularInputTokens < 0 {
		regularInputTokens = 0
	}

	cost := (float64(regularInputTokens)*pricing.InputPrice +
		float64(stats.OutputTokens)*pricing.OutputPrice +
		float64(stats.CachedTokens)*pricing.CachedInputPrice) / 1_000_000

	return math.Round(cost*100) / 100
}

func (u *AggregatedUsage) ToJSON() *AggregatedUsageJSON {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	result := &AggregatedUsageJSON{
		LastUpdated:  u.LastUpdated,
		Combinations: make([]CostCombinationJSON, len(u.Combinations)),
		Costs: CostsSummaryJSON{
			TotalUSD: 0,
			ByUser:   make(map[string]float64),
		},
	}

	for i, combo := range u.Combinations {
		totalCost := calculateCost(combo.Total, combo.Model)

		result.Costs.TotalUSD += totalCost

		comboJSON := CostCombinationJSON{
			Model: combo.Model,
			Total: UsageStatsJSON{
				RequestCount: combo.Total.RequestCount,
				InputTokens:  combo.Total.InputTokens,
				OutputTokens: combo.Total.OutputTokens,
				CachedTokens: combo.Total.CachedTokens,
				CostUSD:      totalCost,
			},
			ByUser: make(map[string]UsageStatsJSON),
		}

		for user, userStats := range combo.ByUser {
			userCost := calculateCost(userStats, combo.Model)
			result.Costs.ByUser[user] += userCost

			comboJSON.ByUser[user] = UsageStatsJSON{
				RequestCount: userStats.RequestCount,
				InputTokens:  userStats.InputTokens,
				OutputTokens: userStats.OutputTokens,
				CachedTokens: userStats.CachedTokens,
				CostUSD:      userCost,
			}
		}

		result.Combinations[i] = comboJSON
	}

	result.Costs.TotalUSD = math.Round(result.Costs.TotalUSD*100) / 100
	for user, cost := range result.Costs.ByUser {
		result.Costs.ByUser[user] = math.Round(cost*100) / 100
	}

	return result
}

func (u *AggregatedUsage) Load() error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

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

	for i := range u.Combinations {
		if u.Combinations[i].ByUser == nil {
			u.Combinations[i].ByUser = make(map[string]UsageStats)
		}
	}

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

func (u *AggregatedUsage) AddUsage(model string, inputTokens, outputTokens, cachedTokens int64, user string) error {
	if model == "" {
		return E.New("model cannot be empty")
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.LastUpdated = time.Now()

	var combo *CostCombination
	for i := range u.Combinations {
		if u.Combinations[i].Model == model {
			combo = &u.Combinations[i]
			break
		}
	}

	if combo == nil {
		newCombo := CostCombination{
			Model:  model,
			Total:  UsageStats{},
			ByUser: make(map[string]UsageStats),
		}
		u.Combinations = append(u.Combinations, newCombo)
		combo = &u.Combinations[len(u.Combinations)-1]
	}

	combo.Total.RequestCount++
	combo.Total.InputTokens += inputTokens
	combo.Total.OutputTokens += outputTokens
	combo.Total.CachedTokens += cachedTokens

	if user != "" {
		userStats := combo.ByUser[user]
		userStats.RequestCount++
		userStats.InputTokens += inputTokens
		userStats.OutputTokens += outputTokens
		userStats.CachedTokens += cachedTokens
		combo.ByUser[user] = userStats
	}

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

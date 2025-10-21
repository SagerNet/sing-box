package ccm

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
	RequestCount             int   `json:"request_count"`
	MessagesCount            int   `json:"messages_count"`
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

type CostCombination struct {
	Model         string                `json:"model"`
	ContextWindow int                   `json:"context_window"`
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
	RequestCount             int     `json:"request_count"`
	MessagesCount            int     `json:"messages_count"`
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

type CostCombinationJSON struct {
	Model         string                    `json:"model"`
	ContextWindow int                       `json:"context_window"`
	Total         UsageStatsJSON            `json:"total"`
	ByUser        map[string]UsageStatsJSON `json:"by_user"`
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
	InputPrice      float64
	OutputPrice     float64
	CacheReadPrice  float64
	CacheWritePrice float64
}

type modelFamily struct {
	pattern         *regexp.Regexp
	standardPricing ModelPricing
	premiumPricing  *ModelPricing
}

var (
	opus4Pricing = ModelPricing{
		InputPrice:      15.0,
		OutputPrice:     75.0,
		CacheReadPrice:  1.5,
		CacheWritePrice: 18.75,
	}

	sonnet4StandardPricing = ModelPricing{
		InputPrice:      3.0,
		OutputPrice:     15.0,
		CacheReadPrice:  0.3,
		CacheWritePrice: 3.75,
	}

	sonnet4PremiumPricing = ModelPricing{
		InputPrice:      6.0,
		OutputPrice:     22.5,
		CacheReadPrice:  0.6,
		CacheWritePrice: 7.5,
	}

	haiku4Pricing = ModelPricing{
		InputPrice:      1.0,
		OutputPrice:     5.0,
		CacheReadPrice:  0.1,
		CacheWritePrice: 1.25,
	}

	haiku35Pricing = ModelPricing{
		InputPrice:      0.8,
		OutputPrice:     4.0,
		CacheReadPrice:  0.08,
		CacheWritePrice: 1.0,
	}

	sonnet35Pricing = ModelPricing{
		InputPrice:      3.0,
		OutputPrice:     15.0,
		CacheReadPrice:  0.3,
		CacheWritePrice: 3.75,
	}

	modelFamilies = []modelFamily{
		{
			pattern:         regexp.MustCompile(`^claude-(?:opus-4-|4-opus-|opus-4-1-)`),
			standardPricing: opus4Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-7-sonnet-`),
			standardPricing: sonnet4StandardPricing,
			premiumPricing:  &sonnet4PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:sonnet-4-|4-sonnet-)`),
			standardPricing: sonnet4StandardPricing,
			premiumPricing:  &sonnet4PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-haiku-4-`),
			standardPricing: haiku4Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-5-haiku-`),
			standardPricing: haiku35Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-5-sonnet-`),
			standardPricing: sonnet35Pricing,
			premiumPricing:  nil,
		},
	}
)

func getPricing(model string, contextWindow int) ModelPricing {
	isPremium := contextWindow >= contextWindowPremium

	for _, family := range modelFamilies {
		if family.pattern.MatchString(model) {
			if isPremium && family.premiumPricing != nil {
				return *family.premiumPricing
			}
			return family.standardPricing
		}
	}

	return sonnet4StandardPricing
}

func calculateCost(stats UsageStats, model string, contextWindow int) float64 {
	pricing := getPricing(model, contextWindow)

	cost := (float64(stats.InputTokens)*pricing.InputPrice +
		float64(stats.OutputTokens)*pricing.OutputPrice +
		float64(stats.CacheReadInputTokens)*pricing.CacheReadPrice +
		float64(stats.CacheCreationInputTokens)*pricing.CacheWritePrice) / 1_000_000

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
		totalCost := calculateCost(combo.Total, combo.Model, combo.ContextWindow)

		result.Costs.TotalUSD += totalCost

		comboJSON := CostCombinationJSON{
			Model:         combo.Model,
			ContextWindow: combo.ContextWindow,
			Total: UsageStatsJSON{
				RequestCount:             combo.Total.RequestCount,
				MessagesCount:            combo.Total.MessagesCount,
				InputTokens:              combo.Total.InputTokens,
				OutputTokens:             combo.Total.OutputTokens,
				CacheReadInputTokens:     combo.Total.CacheReadInputTokens,
				CacheCreationInputTokens: combo.Total.CacheCreationInputTokens,
				CostUSD:                  totalCost,
			},
			ByUser: make(map[string]UsageStatsJSON),
		}

		for user, userStats := range combo.ByUser {
			userCost := calculateCost(userStats, combo.Model, combo.ContextWindow)
			result.Costs.ByUser[user] += userCost

			comboJSON.ByUser[user] = UsageStatsJSON{
				RequestCount:             userStats.RequestCount,
				MessagesCount:            userStats.MessagesCount,
				InputTokens:              userStats.InputTokens,
				OutputTokens:             userStats.OutputTokens,
				CacheReadInputTokens:     userStats.CacheReadInputTokens,
				CacheCreationInputTokens: userStats.CacheCreationInputTokens,
				CostUSD:                  userCost,
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

func (u *AggregatedUsage) AddUsage(model string, contextWindow int, messagesCount int, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64, user string) error {
	if model == "" {
		return E.New("model cannot be empty")
	}
	if contextWindow <= 0 {
		return E.New("contextWindow must be positive")
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.LastUpdated = time.Now()

	// Find or create combination
	var combo *CostCombination
	for i := range u.Combinations {
		if u.Combinations[i].Model == model && u.Combinations[i].ContextWindow == contextWindow {
			combo = &u.Combinations[i]
			break
		}
	}

	if combo == nil {
		newCombo := CostCombination{
			Model:         model,
			ContextWindow: contextWindow,
			Total:         UsageStats{},
			ByUser:        make(map[string]UsageStats),
		}
		u.Combinations = append(u.Combinations, newCombo)
		combo = &u.Combinations[len(u.Combinations)-1]
	}

	// Update total stats
	combo.Total.RequestCount++
	combo.Total.MessagesCount += messagesCount
	combo.Total.InputTokens += inputTokens
	combo.Total.OutputTokens += outputTokens
	combo.Total.CacheReadInputTokens += cacheReadTokens
	combo.Total.CacheCreationInputTokens += cacheCreationTokens

	// Update per-user stats if user is specified
	if user != "" {
		userStats := combo.ByUser[user]
		userStats.RequestCount++
		userStats.MessagesCount += messagesCount
		userStats.InputTokens += inputTokens
		userStats.OutputTokens += outputTokens
		userStats.CacheReadInputTokens += cacheReadTokens
		userStats.CacheCreationInputTokens += cacheCreationTokens
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

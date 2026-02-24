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
	RequestCount                    int   `json:"request_count"`
	MessagesCount                   int   `json:"messages_count"`
	InputTokens                     int64 `json:"input_tokens"`
	OutputTokens                    int64 `json:"output_tokens"`
	CacheReadInputTokens            int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens        int64 `json:"cache_creation_input_tokens"`
	CacheCreation5MinuteInputTokens int64 `json:"cache_creation_5m_input_tokens,omitempty"`
	CacheCreation1HourInputTokens   int64 `json:"cache_creation_1h_input_tokens,omitempty"`
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
	RequestCount                    int     `json:"request_count"`
	MessagesCount                   int     `json:"messages_count"`
	InputTokens                     int64   `json:"input_tokens"`
	OutputTokens                    int64   `json:"output_tokens"`
	CacheReadInputTokens            int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens        int64   `json:"cache_creation_input_tokens"`
	CacheCreation5MinuteInputTokens int64   `json:"cache_creation_5m_input_tokens,omitempty"`
	CacheCreation1HourInputTokens   int64   `json:"cache_creation_1h_input_tokens,omitempty"`
	CostUSD                         float64 `json:"cost_usd"`
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
	InputPrice             float64
	OutputPrice            float64
	CacheReadPrice         float64
	CacheWritePrice5Minute float64
	CacheWritePrice1Hour   float64
}

type modelFamily struct {
	pattern         *regexp.Regexp
	standardPricing ModelPricing
	premiumPricing  *ModelPricing
}

var (
	opus46StandardPricing = ModelPricing{
		InputPrice:             5.0,
		OutputPrice:            25.0,
		CacheReadPrice:         0.5,
		CacheWritePrice5Minute: 6.25,
		CacheWritePrice1Hour:   10.0,
	}

	opus46PremiumPricing = ModelPricing{
		InputPrice:             10.0,
		OutputPrice:            37.5,
		CacheReadPrice:         1.0,
		CacheWritePrice5Minute: 12.5,
		CacheWritePrice1Hour:   20.0,
	}

	opus45Pricing = ModelPricing{
		InputPrice:             5.0,
		OutputPrice:            25.0,
		CacheReadPrice:         0.5,
		CacheWritePrice5Minute: 6.25,
		CacheWritePrice1Hour:   10.0,
	}

	opus4Pricing = ModelPricing{
		InputPrice:             15.0,
		OutputPrice:            75.0,
		CacheReadPrice:         1.5,
		CacheWritePrice5Minute: 18.75,
		CacheWritePrice1Hour:   30.0,
	}

	sonnet46StandardPricing = ModelPricing{
		InputPrice:             3.0,
		OutputPrice:            15.0,
		CacheReadPrice:         0.3,
		CacheWritePrice5Minute: 3.75,
		CacheWritePrice1Hour:   6.0,
	}

	sonnet46PremiumPricing = ModelPricing{
		InputPrice:             6.0,
		OutputPrice:            22.5,
		CacheReadPrice:         0.6,
		CacheWritePrice5Minute: 7.5,
		CacheWritePrice1Hour:   12.0,
	}

	sonnet45StandardPricing = ModelPricing{
		InputPrice:             3.0,
		OutputPrice:            15.0,
		CacheReadPrice:         0.3,
		CacheWritePrice5Minute: 3.75,
		CacheWritePrice1Hour:   6.0,
	}

	sonnet45PremiumPricing = ModelPricing{
		InputPrice:             6.0,
		OutputPrice:            22.5,
		CacheReadPrice:         0.6,
		CacheWritePrice5Minute: 7.5,
		CacheWritePrice1Hour:   12.0,
	}

	sonnet4StandardPricing = ModelPricing{
		InputPrice:             3.0,
		OutputPrice:            15.0,
		CacheReadPrice:         0.3,
		CacheWritePrice5Minute: 3.75,
		CacheWritePrice1Hour:   6.0,
	}

	sonnet4PremiumPricing = ModelPricing{
		InputPrice:             6.0,
		OutputPrice:            22.5,
		CacheReadPrice:         0.6,
		CacheWritePrice5Minute: 7.5,
		CacheWritePrice1Hour:   12.0,
	}

	sonnet37Pricing = ModelPricing{
		InputPrice:             3.0,
		OutputPrice:            15.0,
		CacheReadPrice:         0.3,
		CacheWritePrice5Minute: 3.75,
		CacheWritePrice1Hour:   6.0,
	}

	sonnet35Pricing = ModelPricing{
		InputPrice:             3.0,
		OutputPrice:            15.0,
		CacheReadPrice:         0.3,
		CacheWritePrice5Minute: 3.75,
		CacheWritePrice1Hour:   6.0,
	}

	haiku45Pricing = ModelPricing{
		InputPrice:             1.0,
		OutputPrice:            5.0,
		CacheReadPrice:         0.1,
		CacheWritePrice5Minute: 1.25,
		CacheWritePrice1Hour:   2.0,
	}

	haiku4Pricing = ModelPricing{
		InputPrice:             1.0,
		OutputPrice:            5.0,
		CacheReadPrice:         0.1,
		CacheWritePrice5Minute: 1.25,
		CacheWritePrice1Hour:   2.0,
	}

	haiku35Pricing = ModelPricing{
		InputPrice:             0.8,
		OutputPrice:            4.0,
		CacheReadPrice:         0.08,
		CacheWritePrice5Minute: 1.0,
		CacheWritePrice1Hour:   1.6,
	}

	haiku3Pricing = ModelPricing{
		InputPrice:             0.25,
		OutputPrice:            1.25,
		CacheReadPrice:         0.03,
		CacheWritePrice5Minute: 0.3,
		CacheWritePrice1Hour:   0.5,
	}

	opus3Pricing = ModelPricing{
		InputPrice:             15.0,
		OutputPrice:            75.0,
		CacheReadPrice:         1.5,
		CacheWritePrice5Minute: 18.75,
		CacheWritePrice1Hour:   30.0,
	}

	modelFamilies = []modelFamily{
		{
			pattern:         regexp.MustCompile(`^claude-opus-4-6(?:-|$)`),
			standardPricing: opus46StandardPricing,
			premiumPricing:  &opus46PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-opus-4-5(?:-|$)`),
			standardPricing: opus45Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:opus-4(?:-|$)|4-opus-)`),
			standardPricing: opus4Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:opus-3(?:-|$)|3-opus-)`),
			standardPricing: opus3Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:sonnet-4-6(?:-|$)|4-6-sonnet-)`),
			standardPricing: sonnet46StandardPricing,
			premiumPricing:  &sonnet46PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:sonnet-4-5(?:-|$)|4-5-sonnet-)`),
			standardPricing: sonnet45StandardPricing,
			premiumPricing:  &sonnet45PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:sonnet-4(?:-|$)|4-sonnet-)`),
			standardPricing: sonnet4StandardPricing,
			premiumPricing:  &sonnet4PremiumPricing,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-7-sonnet(?:-|$)`),
			standardPricing: sonnet37Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-5-sonnet(?:-|$)`),
			standardPricing: sonnet35Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-(?:haiku-4-5(?:-|$)|4-5-haiku-)`),
			standardPricing: haiku45Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-haiku-4(?:-|$)`),
			standardPricing: haiku4Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-5-haiku(?:-|$)`),
			standardPricing: haiku35Pricing,
			premiumPricing:  nil,
		},
		{
			pattern:         regexp.MustCompile(`^claude-3-haiku(?:-|$)`),
			standardPricing: haiku3Pricing,
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

	cacheCreationCost := 0.0
	if stats.CacheCreation5MinuteInputTokens > 0 || stats.CacheCreation1HourInputTokens > 0 {
		cacheCreationCost =
			float64(stats.CacheCreation5MinuteInputTokens)*pricing.CacheWritePrice5Minute +
				float64(stats.CacheCreation1HourInputTokens)*pricing.CacheWritePrice1Hour
	} else {
		// Backward compatibility for usage files generated before TTL split tracking.
		cacheCreationCost = float64(stats.CacheCreationInputTokens) * pricing.CacheWritePrice5Minute
	}

	cost := (float64(stats.InputTokens)*pricing.InputPrice +
		float64(stats.OutputTokens)*pricing.OutputPrice +
		float64(stats.CacheReadInputTokens)*pricing.CacheReadPrice +
		cacheCreationCost) / 1_000_000

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
				RequestCount:                    combo.Total.RequestCount,
				MessagesCount:                   combo.Total.MessagesCount,
				InputTokens:                     combo.Total.InputTokens,
				OutputTokens:                    combo.Total.OutputTokens,
				CacheReadInputTokens:            combo.Total.CacheReadInputTokens,
				CacheCreationInputTokens:        combo.Total.CacheCreationInputTokens,
				CacheCreation5MinuteInputTokens: combo.Total.CacheCreation5MinuteInputTokens,
				CacheCreation1HourInputTokens:   combo.Total.CacheCreation1HourInputTokens,
				CostUSD:                         totalCost,
			},
			ByUser: make(map[string]UsageStatsJSON),
		}

		for user, userStats := range combo.ByUser {
			userCost := calculateCost(userStats, combo.Model, combo.ContextWindow)
			result.Costs.ByUser[user] += userCost

			comboJSON.ByUser[user] = UsageStatsJSON{
				RequestCount:                    userStats.RequestCount,
				MessagesCount:                   userStats.MessagesCount,
				InputTokens:                     userStats.InputTokens,
				OutputTokens:                    userStats.OutputTokens,
				CacheReadInputTokens:            userStats.CacheReadInputTokens,
				CacheCreationInputTokens:        userStats.CacheCreationInputTokens,
				CacheCreation5MinuteInputTokens: userStats.CacheCreation5MinuteInputTokens,
				CacheCreation1HourInputTokens:   userStats.CacheCreation1HourInputTokens,
				CostUSD:                         userCost,
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

func (u *AggregatedUsage) AddUsage(
	model string,
	contextWindow int,
	messagesCount int,
	inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens int64,
	user string,
) error {
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

	if cacheCreationTokens == 0 {
		cacheCreationTokens = cacheCreation5MinuteTokens + cacheCreation1HourTokens
	}

	// Update total stats
	combo.Total.RequestCount++
	combo.Total.MessagesCount += messagesCount
	combo.Total.InputTokens += inputTokens
	combo.Total.OutputTokens += outputTokens
	combo.Total.CacheReadInputTokens += cacheReadTokens
	combo.Total.CacheCreationInputTokens += cacheCreationTokens
	combo.Total.CacheCreation5MinuteInputTokens += cacheCreation5MinuteTokens
	combo.Total.CacheCreation1HourInputTokens += cacheCreation1HourTokens

	// Update per-user stats if user is specified
	if user != "" {
		userStats := combo.ByUser[user]
		userStats.RequestCount++
		userStats.MessagesCount += messagesCount
		userStats.InputTokens += inputTokens
		userStats.OutputTokens += outputTokens
		userStats.CacheReadInputTokens += cacheReadTokens
		userStats.CacheCreationInputTokens += cacheCreationTokens
		userStats.CacheCreation5MinuteInputTokens += cacheCreation5MinuteTokens
		userStats.CacheCreation1HourInputTokens += cacheCreation1HourTokens
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

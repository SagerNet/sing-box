package ccm

import (
	"encoding/json"
	"fmt"
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
		cacheCreationCost = float64(stats.CacheCreation5MinuteInputTokens)*pricing.CacheWritePrice5Minute +
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

func roundCost(cost float64) float64 {
	return math.Round(cost*100) / 100
}

func normalizeCombinations(combinations []CostCombination) {
	for index := range combinations {
		if combinations[index].ByUser == nil {
			combinations[index].ByUser = make(map[string]UsageStats)
		}
	}
}

func addUsageToCombinations(
	combinations *[]CostCombination,
	model string,
	contextWindow int,
	weekStartUnix int64,
	messagesCount int,
	inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens int64,
	user string,
) {
	var matchedCombination *CostCombination
	for index := range *combinations {
		combination := &(*combinations)[index]
		if combination.Model == model && combination.ContextWindow == contextWindow && combination.WeekStartUnix == weekStartUnix {
			matchedCombination = combination
			break
		}
	}

	if matchedCombination == nil {
		newCombination := CostCombination{
			Model:         model,
			ContextWindow: contextWindow,
			WeekStartUnix: weekStartUnix,
			Total:         UsageStats{},
			ByUser:        make(map[string]UsageStats),
		}
		*combinations = append(*combinations, newCombination)
		matchedCombination = &(*combinations)[len(*combinations)-1]
	}

	if cacheCreationTokens == 0 {
		cacheCreationTokens = cacheCreation5MinuteTokens + cacheCreation1HourTokens
	}

	matchedCombination.Total.RequestCount++
	matchedCombination.Total.MessagesCount += messagesCount
	matchedCombination.Total.InputTokens += inputTokens
	matchedCombination.Total.OutputTokens += outputTokens
	matchedCombination.Total.CacheReadInputTokens += cacheReadTokens
	matchedCombination.Total.CacheCreationInputTokens += cacheCreationTokens
	matchedCombination.Total.CacheCreation5MinuteInputTokens += cacheCreation5MinuteTokens
	matchedCombination.Total.CacheCreation1HourInputTokens += cacheCreation1HourTokens

	if user != "" {
		userStats := matchedCombination.ByUser[user]
		userStats.RequestCount++
		userStats.MessagesCount += messagesCount
		userStats.InputTokens += inputTokens
		userStats.OutputTokens += outputTokens
		userStats.CacheReadInputTokens += cacheReadTokens
		userStats.CacheCreationInputTokens += cacheCreationTokens
		userStats.CacheCreation5MinuteInputTokens += cacheCreation5MinuteTokens
		userStats.CacheCreation1HourInputTokens += cacheCreation1HourTokens
		matchedCombination.ByUser[user] = userStats
	}
}

func buildCombinationJSON(combinations []CostCombination, aggregateUserCosts map[string]float64) ([]CostCombinationJSON, float64) {
	result := make([]CostCombinationJSON, len(combinations))
	var totalCost float64

	for index, combination := range combinations {
		combinationTotalCost := calculateCost(combination.Total, combination.Model, combination.ContextWindow)
		totalCost += combinationTotalCost

		combinationJSON := CostCombinationJSON{
			Model:         combination.Model,
			ContextWindow: combination.ContextWindow,
			WeekStartUnix: combination.WeekStartUnix,
			Total: UsageStatsJSON{
				RequestCount:                    combination.Total.RequestCount,
				MessagesCount:                   combination.Total.MessagesCount,
				InputTokens:                     combination.Total.InputTokens,
				OutputTokens:                    combination.Total.OutputTokens,
				CacheReadInputTokens:            combination.Total.CacheReadInputTokens,
				CacheCreationInputTokens:        combination.Total.CacheCreationInputTokens,
				CacheCreation5MinuteInputTokens: combination.Total.CacheCreation5MinuteInputTokens,
				CacheCreation1HourInputTokens:   combination.Total.CacheCreation1HourInputTokens,
				CostUSD:                         combinationTotalCost,
			},
			ByUser: make(map[string]UsageStatsJSON),
		}

		for user, userStats := range combination.ByUser {
			userCost := calculateCost(userStats, combination.Model, combination.ContextWindow)
			if aggregateUserCosts != nil {
				aggregateUserCosts[user] += userCost
			}

			combinationJSON.ByUser[user] = UsageStatsJSON{
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
		byWeek[weekKey] += calculateCost(combination.Total, combination.Model, combination.ContextWindow)
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

func (u *AggregatedUsage) AddUsage(
	model string,
	contextWindow int,
	messagesCount int,
	inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens int64,
	user string,
) error {
	return u.AddUsageWithCycleHint(model, contextWindow, messagesCount, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens, user, time.Now(), nil)
}

func (u *AggregatedUsage) AddUsageWithCycleHint(
	model string,
	contextWindow int,
	messagesCount int,
	inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens int64,
	user string,
	observedAt time.Time,
	cycleHint *WeeklyCycleHint,
) error {
	if model == "" {
		return E.New("model cannot be empty")
	}
	if contextWindow <= 0 {
		return E.New("contextWindow must be positive")
	}
	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.LastUpdated = observedAt
	weekStartUnix := deriveWeekStartUnix(cycleHint)

	addUsageToCombinations(&u.Combinations, model, contextWindow, weekStartUnix, messagesCount, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheCreation5MinuteTokens, cacheCreation1HourTokens, user)

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

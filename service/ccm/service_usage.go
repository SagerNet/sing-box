package ccm

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"sync"
	"time"
)

// In-memory structures - NO cost_usd fields
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
	mu           sync.Mutex
	filePath     string
}

// JSON output structures - WITH cost_usd fields calculated on save
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

// Pricing structure per million tokens
type ModelPricing struct {
	InputPrice      float64
	OutputPrice     float64
	CacheReadPrice  float64
	CacheWritePrice float64
}

// getPricing returns pricing for a given model and context window (in USD per million tokens)
func getPricing(model string, contextWindow int) ModelPricing {
	modelLower := strings.ToLower(model)

	// Determine if this is premium pricing (1M context with >200K tokens)
	isPremium := contextWindow >= 1000000

	// Opus 4/4.1
	if strings.Contains(modelLower, "opus-4") {
		return ModelPricing{
			InputPrice:      15.0,
			OutputPrice:     75.0,
			CacheReadPrice:  1.5,
			CacheWritePrice: 18.75, // 5m cache write
		}
	}

	// Sonnet 4/4.5/3.7
	if strings.Contains(modelLower, "sonnet-4") || strings.Contains(modelLower, "sonnet-3-7") {
		if isPremium {
			return ModelPricing{
				InputPrice:      6.0,
				OutputPrice:     22.5,
				CacheReadPrice:  0.6,
				CacheWritePrice: 7.5, // 5m cache write
			}
		}
		return ModelPricing{
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheReadPrice:  0.3,
			CacheWritePrice: 3.75, // 5m cache write
		}
	}

	// Haiku 4.5
	if strings.Contains(modelLower, "haiku-4") {
		return ModelPricing{
			InputPrice:      1.0,
			OutputPrice:     5.0,
			CacheReadPrice:  0.1,
			CacheWritePrice: 1.25, // 5m cache write
		}
	}

	// Haiku 3.5
	if strings.Contains(modelLower, "haiku-3") {
		return ModelPricing{
			InputPrice:      0.8,
			OutputPrice:     4.0,
			CacheReadPrice:  0.08,
			CacheWritePrice: 1.0, // 5m cache write
		}
	}

	// Sonnet 3.5 (legacy)
	if strings.Contains(modelLower, "sonnet-3-5") || strings.Contains(modelLower, "sonnet-3.5") {
		return ModelPricing{
			InputPrice:      3.0,
			OutputPrice:     15.0,
			CacheReadPrice:  0.3,
			CacheWritePrice: 3.75,
		}
	}

	// Default to Sonnet 4.5 pricing if unknown
	return ModelPricing{
		InputPrice:      3.0,
		OutputPrice:     15.0,
		CacheReadPrice:  0.3,
		CacheWritePrice: 3.75,
	}
}

// calculateCost calculates cost from token counts, rounded to 2 decimal places
func calculateCost(stats UsageStats, model string, contextWindow int) float64 {
	pricing := getPricing(model, contextWindow)

	cost := (float64(stats.InputTokens)*pricing.InputPrice +
		float64(stats.OutputTokens)*pricing.OutputPrice +
		float64(stats.CacheReadInputTokens)*pricing.CacheReadPrice +
		float64(stats.CacheCreationInputTokens)*pricing.CacheWritePrice) / 1_000_000

	// Round to 2 decimal places
	return math.Round(cost*100) / 100
}

// ToJSON converts in-memory stats to JSON with calculated costs
func (u *AggregatedUsage) ToJSON() *AggregatedUsageJSON {
	u.mu.Lock()
	defer u.mu.Unlock()

	result := &AggregatedUsageJSON{
		LastUpdated:  u.LastUpdated,
		Combinations: make([]CostCombinationJSON, len(u.Combinations)),
		Costs: CostsSummaryJSON{
			TotalUSD: 0,
			ByUser:   make(map[string]float64),
		},
	}

	// Convert each combination and calculate costs
	for i, combo := range u.Combinations {
		totalCost := calculateCost(combo.Total, combo.Model, combo.ContextWindow)

		// Update overall total
		result.Costs.TotalUSD += totalCost

		// Convert combination
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

		// Convert per-user stats and calculate costs
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

	// Round total costs to 2 decimal places
	result.Costs.TotalUSD = math.Round(result.Costs.TotalUSD*100) / 100
	for user, cost := range result.Costs.ByUser {
		result.Costs.ByUser[user] = math.Round(cost*100) / 100
	}

	return result
}

// NewAggregatedUsage creates a new aggregated usage tracker
func NewAggregatedUsage(filePath string) *AggregatedUsage {
	return &AggregatedUsage{
		LastUpdated:  time.Now(),
		Combinations: make([]CostCombination, 0),
		filePath:     filePath,
	}
}

// Load loads usage statistics from file
func (u *AggregatedUsage) Load() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	data, err := os.ReadFile(u.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Load into temporary struct that has all fields
	var temp struct {
		LastUpdated  time.Time         `json:"last_updated"`
		Combinations []CostCombination `json:"combinations"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	u.LastUpdated = temp.LastUpdated
	u.Combinations = temp.Combinations

	// Initialize ByUser maps if nil
	for i := range u.Combinations {
		if u.Combinations[i].ByUser == nil {
			u.Combinations[i].ByUser = make(map[string]UsageStats)
		}
	}

	return nil
}

// Save saves usage statistics to file with calculated costs
func (u *AggregatedUsage) Save() error {
	jsonData := u.ToJSON()

	data, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(u.filePath, data, 0o644)
}

// AddUsage adds usage data for a request
func (u *AggregatedUsage) AddUsage(model string, contextWindow int, messagesCount int, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64, user string) {
	u.mu.Lock()
	defer u.mu.Unlock()

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
}

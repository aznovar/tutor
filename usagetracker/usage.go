package usagetracker

type UsageTracker struct {
	UserID  int
	Name    string
	CostMap map[string]float64
}

// NewUsageTracker создает новый UsageTracker с заданным UserID и именем
func NewUsageTracker(userID int, name string) *UsageTracker {
	return &UsageTracker{
		UserID:  userID,
		Name:    name,
		CostMap: make(map[string]float64),
	}
}

// AddChatTokens добавляет токены для использования в чате
func (ut *UsageTracker) AddChatTokens(tokens int, price float64) {
	ut.CostMap["cost_today"] += float64(tokens) * price
	ut.CostMap["cost_month"] += float64(tokens) * price
	ut.CostMap["cost_all_time"] += float64(tokens) * price
}

// GetCurrentCost возвращает текущие затраты по заданному периоду
func (ut *UsageTracker) GetCurrentCost(period string) float64 {
	return ut.CostMap[period]
}

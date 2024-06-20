package config

type Config struct {
	APIKey                    string
	BotLanguage               string
	Model                     string
	MaxTokens                 int
	NChoices                  int
	Temperature               float32
	PresencePenalty           float32
	FrequencyPenalty          float32
	ShowUsage                 bool
	AdminUserIDs              string
	AllowedUserIDs            string
	UserBudgets               string
	BudgetPeriod              string
	GuestBudget               float64
	EnableQuoting             bool
	TokenPrice                float64
	MaxHistorySize            int
	MaxConversationAgeMinutes int
	AssistantPrompt           string
	ImageSize                 string
}

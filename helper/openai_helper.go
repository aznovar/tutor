package helper

import (
	"encoding/json"
	"log"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

var (
	GPT_3_MODELS     = []string{"gpt-3.5-turbo", "gpt-3.5-turbo-0301", "gpt-3.5-turbo-0613"}
	GPT_3_16K_MODELS = []string{"gpt-3.5-turbo-16k", "gpt-3.5-turbo-16k-0613"}
	GPT_4_MODELS     = []string{"gpt-4", "gpt-4-0314", "gpt-4-0613", "gpt-4-turbo", "gpt-4o"}
	GPT_4_32K_MODELS = []string{"gpt-4-32k", "gpt-4-32k-0314", "gpt-4-32k-0613"}
	GPT_ALL_MODELS   = append(GPT_3_MODELS, append(GPT_3_16K_MODELS, append(GPT_4_MODELS, GPT_4_32K_MODELS...)...)...)
)

type Config struct {
	APIKey                    string
	Proxy                     string
	Model                     string
	MaxTokens                 int
	NChoices                  int
	Temperature               float64
	PresencePenalty           float64
	FrequencyPenalty          float64
	MaxHistorySize            int
	MaxConversationAgeMinutes int
	ShowUsage                 bool
	BotLanguage               string
	AssistantPrompt           string
	ImageSize                 string
}

var translations map[string]map[string]string

func init() {
	parentDirPath := ".."
	translationsFilePath := parentDirPath + "/translations.json"
	data, err := os.ReadFile(translationsFilePath)
	if err != nil {
		log.Fatalf("Error reading translations file: %v", err)
	}
	err = json.Unmarshal(data, &translations)
	if err != nil {
		log.Fatalf("Error unmarshalling translations: %v", err)
	}
}

func localizedText(key, botLanguage string) string {
	if val, ok := translations[botLanguage][key]; ok {
		return val
	}
	log.Printf("No translation available for bot_language code '%s' and key '%s'", botLanguage, key)
	if val, ok := translations["en"][key]; ok {
		return val
	}
	log.Printf("No English definition found for key '%s' in translations.json", key)
	return key
}

func defaultMaxTokens(model string) int {
	base := 1200
	if contains(GPT_3_MODELS, model) {
		return base
	} else if contains(GPT_4_MODELS, model) {
		return base * 2
	} else if contains(GPT_3_16K_MODELS, model) {
		return base * 4
	} else if contains(GPT_4_32K_MODELS, model) {
		return base * 8
	}
	return base
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

type OpenAIHelper struct {
	Client        *openai.Client
	Config        Config
	Conversations map[int][]openai.ChatCompletionMessage
	LastUpdated   map[int]time.Time
}

func NewOpenAIHelper(config Config) *OpenAIHelper {
	client := openai.NewClient(config.APIKey)
	return &OpenAIHelper{
		Client:        client,
		Config:        config,
		Conversations: make(map[int][]openai.ChatCompletionMessage),
		LastUpdated:   make(map[int]time.Time),
	}
}

func (o *OpenAIHelper) ResetChatHistory(chatID int, content string) {
	if content == "" {
		content = o.Config.AssistantPrompt
	}
	o.Conversations[chatID] = []openai.ChatCompletionMessage{{Role: "system", Content: content}}
}

func (o *OpenAIHelper) MaxAgeReached(chatID int) bool {
	lastUpdated, ok := o.LastUpdated[chatID]
	if !ok {
		return false
	}
	return lastUpdated.Before(time.Now().Add(-time.Duration(o.Config.MaxConversationAgeMinutes) * time.Minute))
}

func (o *OpenAIHelper) AddToHistory(chatID int, role, content string) {
	o.Conversations[chatID] = append(o.Conversations[chatID], openai.ChatCompletionMessage{Role: role, Content: content})
}

func (o *OpenAIHelper) GetConversationStats(chatID int) (int, int) {
	if _, ok := o.Conversations[chatID]; !ok {
		o.ResetChatHistory(chatID, "")
	}
	return len(o.Conversations[chatID]), o.CountTokens(o.Conversations[chatID])
}

func (o *OpenAIHelper) CountTokens(messages []openai.ChatCompletionMessage) int {
	// Implement token counting logic here.
	// For simplicity, let's assume each message has a fixed token count (this is not accurate).
	return len(messages) * 5
}

package main

import (
	"encoding/json"
	"log"
	"os"
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

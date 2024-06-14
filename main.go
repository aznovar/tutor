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

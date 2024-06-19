package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkoukk/tiktoken-go"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/pkoukk/tiktoken-go"

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

func (o *OpenAIHelper) MaxModelTokens() int {
	base := 4096
	if contains(GPT_3_MODELS, o.Config.Model) {
		return base
	} else if contains(GPT_3_16K_MODELS, o.Config.Model) {
		return base * 4
	} else if contains(GPT_4_MODELS, o.Config.Model) {
		return base * 2
	} else if contains(GPT_4_32K_MODELS, o.Config.Model) {
		return base * 8
	}
	return base
}

func (o *OpenAIHelper) Summarise(conversation []openai.ChatCompletionMessage) (string, error) {
	// Преобразование массива сообщений в строку
	conversationContent, err := json.Marshal(conversation)
	if err != nil {
		return "", err
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "Summarize this conversation in 700 characters or less"},
		{Role: "user", Content: string(conversationContent)},
	}

	req := openai.ChatCompletionRequest{
		Model:       o.Config.Model,
		Messages:    messages,
		Temperature: 0.4,
	}

	// Добавление контекста к вызову CreateChatCompletion
	ctx := context.Background()
	response, err := o.Client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
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

func (o *OpenAIHelper) GetConversationStats(chatID int) (int, int, error) {
	if _, ok := o.Conversations[chatID]; !ok {
		o.ResetChatHistory(chatID, "")
	}
	tokenCount, err := o.CountTokens(o.Conversations[chatID])
	if err != nil {
		return 0, 0, err
	}
	return len(o.Conversations[chatID]), tokenCount, nil
}

func (o *OpenAIHelper) CountTokens(messages []openai.ChatCompletionMessage) (int, error) {
	model := o.Config.Model
	var encoding *tiktoken.Tiktoken
	var err error

	// Получение кодировки для модели
	encoding, err = tiktoken.EncodingForModel(model)
	if err != nil {
		encoding, err = tiktoken.GetEncoding("gpt-3.5-turbo")
		if err != nil {
			return 0, err
		}
	}

	var tokensPerMessage, tokensPerName int
	if contains(GPT_3_MODELS, model) || contains(GPT_3_16K_MODELS, model) {
		tokensPerMessage = 4 // каждый сообщение следует {role/name}\n{content}\n
		tokensPerName = -1   // если есть имя, роль опущена
	} else if contains(GPT_4_MODELS, model) || contains(GPT_4_32K_MODELS, model) {
		tokensPerMessage = 3
		tokensPerName = 1
	} else {
		return 0, fmt.Errorf("num_tokens_from_messages() is not implemented for model %s", model)
	}

	numTokens := 0
	for _, message := range messages {
		numTokens += tokensPerMessage

		encodedRole := encoding.Encode(message.Role, nil, nil)
		numTokens += len(encodedRole)

		encodedContent := encoding.Encode(message.Content, nil, nil)
		numTokens += len(encodedContent)

		if message.Name != "" {
			encodedName := encoding.Encode(message.Name, nil, nil)
			numTokens += len(encodedName)
			numTokens += tokensPerName
		}
	}
	numTokens += 3 // каждый ответ начинается с assistant

	return numTokens, nil
}

func (o *OpenAIHelper) CommonGetChatResponse(chatID int, query string, stream bool) (*openai.ChatCompletionResponse, error) {
	if _, ok := o.Conversations[chatID]; !ok || o.MaxAgeReached(chatID) {
		o.ResetChatHistory(chatID, "")
	}

	o.LastUpdated[chatID] = time.Now()
	o.AddToHistory(chatID, "user", query)

	tokenCount, err := o.CountTokens(o.Conversations[chatID])
	if err != nil {
		return nil, fmt.Errorf("error counting tokens: %v", err)
	}

	exceededMaxTokens := tokenCount+o.Config.MaxTokens > o.MaxModelTokens()
	exceededMaxHistorySize := len(o.Conversations[chatID]) > o.Config.MaxHistorySize

	if exceededMaxTokens || exceededMaxHistorySize {
		log.Printf("Chat history for chat ID %d is too long. Summarising...", chatID)
		summary, err := o.Summarise(o.Conversations[chatID][:len(o.Conversations[chatID])-1])
		if err != nil {
			log.Printf("Error while summarising chat history: %v. Popping elements instead...", err)
			o.Conversations[chatID] = o.Conversations[chatID][len(o.Conversations[chatID])-o.Config.MaxHistorySize:]
		} else {
			o.ResetChatHistory(chatID, summary)
			o.AddToHistory(chatID, "user", query)
		}
	}

	req := openai.ChatCompletionRequest{
		Model:            o.Config.Model,
		Messages:         o.Conversations[chatID],
		MaxTokens:        o.Config.MaxTokens,
		N:                o.Config.NChoices,
		Temperature:      float32(o.Config.Temperature),
		PresencePenalty:  float32(o.Config.PresencePenalty),
		FrequencyPenalty: float32(o.Config.FrequencyPenalty),
		Stream:           stream,
	}

	ctx := context.Background()

	if stream {
		stream, err := o.Client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			return nil, err
		}
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				return nil, err
			}
			log.Printf("Stream response: %v", response)
			if response.Choices[0].FinishReason != "" {
				break
			}
		}
		return nil, nil
	} else {
		response, err := o.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func (o *OpenAIHelper) GenerateImage(prompt string) (string, string, error) {
	botLanguage := o.Config.BotLanguage
	response, err := o.Client.CreateImage(context.Background(), openai.ImageRequest{
		Prompt: prompt,
		N:      1,
		Size:   o.Config.ImageSize,
	})
	if err != nil {
		return "", "", err
	}

	if len(response.Data) == 0 {
		log.Printf("No response from GPT: %v", response)
		return "", "", fmt.Errorf("⚠️ _%s._ ⚠️\n%s.",
			localizedText("error", botLanguage),
			localizedText("try_again", botLanguage))
	}

	return response.Data[0].URL, o.Config.ImageSize, nil
}

func (o *OpenAIHelper) GetChatResponseStream(chatID int, query string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		defer close(responseChan)
		defer close(errorChan)

		ctx := context.Background()
		req := openai.ChatCompletionRequest{
			Model:            o.Config.Model,
			Messages:         o.Conversations[chatID],
			MaxTokens:        o.Config.MaxTokens,
			N:                o.Config.NChoices,
			Temperature:      float32(o.Config.Temperature),
			PresencePenalty:  float32(o.Config.PresencePenalty),
			FrequencyPenalty: float32(o.Config.FrequencyPenalty),
			Stream:           true,
		}

		stream, err := o.Client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			errorChan <- err
			return
		}
		defer stream.Close()

		answer := ""
		for {
			response, err := stream.Recv()
			if err != nil {
				errorChan <- err
				return
			}
			if len(response.Choices) == 0 {
				continue
			}
			delta := response.Choices[0].Delta
			if content := delta.Content; content != "" {
				answer += content
				responseChan <- answer
			}
			if response.Choices[0].FinishReason != "" {
				break
			}
		}

		answer = strings.TrimSpace(answer)
		o.AddToHistory(chatID, "assistant", answer)
		tokensUsed, err := o.CountTokens(o.Conversations[chatID])
		if err != nil {
			errorChan <- err
			return
		}

		if o.Config.ShowUsage {
			responseChan <- fmt.Sprintf("%s\n\n---\n💰 %d %s", answer, tokensUsed, localizedText("stats_tokens", o.Config.BotLanguage))
		} else {
			responseChan <- answer
		}
	}()

	return responseChan, errorChan
}

func (o *OpenAIHelper) GetBillingCurrentMonth() (float64, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + o.Config.APIKey,
	}
	today := time.Now()
	firstDay := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	lastDay := firstDay.AddDate(0, 1, -1)

	params := fmt.Sprintf("?start_date=%s&end_date=%s", firstDay.Format("2006-01-02"), lastDay.Format("2006-01-02"))

	req, err := http.NewRequest("GET", "https://api.openai.com/dashboard/billing/usage"+params, nil)
	if err != nil {
		return 0, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var billingData map[string]interface{}
	err = json.Unmarshal(body, &billingData)
	if err != nil {
		return 0, err
	}

	usageMonth := billingData["total_usage"].(float64) / 100 // convert cent amount to dollars
	return usageMonth, nil
}

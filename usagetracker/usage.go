package usagetracker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type UsageTracker struct {
	UserID       int
	Name         string
	CostMap      map[string]float64
	UserFile     string
	Usage        map[string]interface{}
	UsageHistory map[string]map[string]interface{}
}

// NewUsageTracker создает новый UsageTracker с заданным UserID и именем
func NewUsageTracker(userID int, userName string, logsDir string) *UsageTracker {
	userFile := filepath.Join(logsDir, fmt.Sprintf("%d.json", userID))

	usage := make(map[string]interface{})
	if _, err := os.Stat(userFile); err == nil {
		data, err := ioutil.ReadFile(userFile)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(data, &usage)
		if err != nil {
			panic(err)
		}
	} else {
		os.MkdirAll(logsDir, os.ModePerm)
		usage = map[string]interface{}{
			"user_name": userName,
			"current_cost": map[string]interface{}{
				"day":         0.0,
				"month":       0.0,
				"all_time":    0.0,
				"last_update": time.Now().Format("2006-01-02"),
			},
			"usage_history": map[string]interface{}{
				"chat_tokens":           make(map[string]int),
				"transcription_seconds": make(map[string]int),
				"number_images":         make(map[string][]int),
			},
		}
	}

	return &UsageTracker{
		UserID:       userID,
		Name:         userName,
		UserFile:     userFile,
		Usage:        usage,
		CostMap:      usage["current_cost"].(map[string]float64),
		UsageHistory: usage["usage_history"].(map[string]map[string]interface{}),
	}
}

// AddChatTokens добавляет использованные токены в историю использования и обновляет текущую стоимость
func (ut *UsageTracker) AddChatTokens(tokens int, tokensPrice float64) {
	today := time.Now().Format("2006-01-02")
	tokenCost := round(float64(tokens)*tokensPrice/1000, 6)
	ut.AddCurrentCosts(tokenCost)

	if val, ok := ut.UsageHistory["chat_tokens"][today]; ok {
		ut.UsageHistory["chat_tokens"][today] = val.(int) + tokens
	} else {
		ut.UsageHistory["chat_tokens"][today] = tokens
	}

	ut.saveUsage()
}

// GetCurrentCost возвращает общую сумму затрат за текущий день и месяц
func (ut *UsageTracker) GetCurrentCost() map[string]float64 {
	today := time.Now().Format("2006-01-02")
	lastUpdate := ut.Usage["current_cost"].(map[string]interface{})["last_update"].(string)

	costDay := 0.0
	costMonth := 0.0
	if today == lastUpdate {
		costDay = ut.CostMap["day"]
		costMonth = ut.CostMap["month"]
	} else {
		if yearMonth(time.Now()) == yearMonth(parseDate(lastUpdate)) {
			costMonth = ut.CostMap["month"]
		}
		costDay = 0.0
		costMonth = 0.0
	}
	costAllTime := ut.CostMap["all_time"]

	return map[string]float64{"cost_today": costDay, "cost_month": costMonth, "cost_all_time": costAllTime}
}

// yearMonth возвращает строку в формате год-месяц, например "2023-03"
func yearMonth(date time.Time) string {
	return date.Format("2006-01")
}

func (ut *UsageTracker) saveUsage() {
	data, err := json.MarshalIndent(ut.Usage, "", "  ")
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(ut.UserFile, data, os.ModePerm)
	if err != nil {
		panic(err)
	}
}

// GetCurrentTokenUsage возвращает количество использованных токенов за сегодня и за этот месяц
func (ut *UsageTracker) GetCurrentTokenUsage() (int, int) {
	today := time.Now().Format("2006-01-02")
	month := yearMonth(time.Now())

	usageDay := 0
	if val, ok := ut.UsageHistory["chat_tokens"][today]; ok {
		usageDay = val.(int)
	}

	usageMonth := 0
	for dateStr, tokens := range ut.UsageHistory["chat_tokens"] {
		if strings.HasPrefix(dateStr, month) {
			usageMonth += tokens.(int)
		}
	}

	return usageDay, usageMonth
}

// AddImageRequest добавляет запрос изображения в историю использования и обновляет текущие затраты
func (ut *UsageTracker) AddImageRequest(imageSize string, imagePrices []float64) {
	sizes := []string{"256x256", "512x512", "1024x1024"}
	requestedSize := indexOf(sizes, imageSize)
	if requestedSize == -1 {
		panic("invalid image size")
	}
	imageCost := imagePrices[requestedSize]
	today := time.Now().Format("2006-01-02")
	ut.AddCurrentCosts(imageCost)

	if val, ok := ut.UsageHistory["number_images"][today]; ok {
		val.([]int)[requestedSize]++
	} else {
		ut.UsageHistory["number_images"][today] = []int{0, 0, 0}
		ut.UsageHistory["number_images"][today].([]int)[requestedSize]++
	}

	ut.saveUsage()
}

// GetCurrentImageCount возвращает количество изображений, запрошенных за сегодня и за этот месяц
func (ut *UsageTracker) GetCurrentImageCount() (int, int) {
	today := time.Now().Format("2006-01-02")
	month := yearMonth(time.Now())

	usageDay := 0
	if val, ok := ut.UsageHistory["number_images"][today]; ok {
		usageDay = sum(val.([]int))
	}

	usageMonth := 0
	for dateStr, images := range ut.UsageHistory["number_images"] {
		if strings.HasPrefix(dateStr, month) {
			usageMonth += sum(images.([]int))
		}
	}

	return usageDay, usageMonth
}

// AddTranscriptionSeconds добавляет запрошенные секунды транскрипции в историю использования и обновляет текущие затраты
func (ut *UsageTracker) AddTranscriptionSeconds(seconds int, minutePrice float64) {
	today := time.Now().Format("2006-01-02")
	transcriptionPrice := round(float64(seconds)*minutePrice/60, 2)
	ut.AddCurrentCosts(transcriptionPrice)

	if val, ok := ut.UsageHistory["transcription_seconds"][today]; ok {
		ut.UsageHistory["transcription_seconds"][today] = val.(int) + seconds
	} else {
		ut.UsageHistory["transcription_seconds"][today] = seconds
	}

	ut.saveUsage()
}

// AddCurrentCosts добавляет текущие затраты к общим затратам за все время, день и месяц
func (ut *UsageTracker) AddCurrentCosts(requestCost float64) {
	today := time.Now().Format("2006-01-02")
	lastUpdate := ut.Usage["current_cost"].(map[string]interface{})["last_update"].(string)

	// add to all_time cost, initialize with calculation of total_cost if key doesn't exist
	ut.CostMap["all_time"] += requestCost
	if today == lastUpdate {
		ut.CostMap["day"] += requestCost
		ut.CostMap["month"] += requestCost
	} else {
		if yearMonth(time.Now()) == yearMonth(parseDate(lastUpdate)) {
			ut.CostMap["month"] += requestCost
		} else {
			ut.CostMap["month"] = requestCost
		}
		ut.CostMap["day"] = requestCost
		ut.Usage["current_cost"].(map[string]interface{})["last_update"] = today
	}
}

// GetCurrentTranscriptionDuration возвращает минуты и секунды аудио, транскрибированные за сегодня и за этот месяц
func (ut *UsageTracker) GetCurrentTranscriptionDuration() (int, float64, int, float64) {
	today := time.Now().Format("2006-01-02")
	month := yearMonth(time.Now())

	secondsDay := 0
	if val, ok := ut.UsageHistory["transcription_seconds"][today]; ok {
		secondsDay = val.(int)
	}

	secondsMonth := 0
	for dateStr, seconds := range ut.UsageHistory["transcription_seconds"] {
		if strings.HasPrefix(dateStr, month) {
			secondsMonth += seconds.(int)
		}
	}

	minutesDay, remainingSecondsDay := divmod(secondsDay, 60)
	minutesMonth, remainingSecondsMonth := divmod(secondsMonth, 60)

	return minutesDay, round(remainingSecondsDay, 2), minutesMonth, round(remainingSecondsMonth, 2)
}

// InitializeAllTimeCost возвращает общую сумму затрат всех запросов в истории
func (ut *UsageTracker) InitializeAllTimeCost(tokensPrice float64, imagePrices []float64, minutePrice float64) float64 {
	totalTokens := 0
	for _, tokens := range ut.UsageHistory["chat_tokens"] {
		totalTokens += tokens.(int)
	}
	tokenCost := round(float64(totalTokens)*tokensPrice/1000, 6)

	totalImages := []int{0, 0, 0}
	for _, images := range ut.UsageHistory["number_images"] {
		for i, val := range images.([]int) {
			totalImages[i] += val
		}
	}
	imageCost := 0.0
	for i, count := range totalImages {
		imageCost += float64(count) * imagePrices[i]
	}

	totalTranscriptionSeconds := 0
	for _, seconds := range ut.UsageHistory["transcription_seconds"] {
		totalTranscriptionSeconds += seconds.(int)
	}
	transcriptionCost := round(float64(totalTranscriptionSeconds)*minutePrice/60, 2)

	allTimeCost := tokenCost + transcriptionCost + imageCost
	return allTimeCost
}

// Вспомогательные функции

func round(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

func sum(slice []int) int {
	total := 0
	for _, v := range slice {
		total += v
	}
	return total
}

func divmod(val, div int) (int, float64) {
	quotient := val / div
	remainder := val % div
	return quotient, float64(remainder)
}

func parseDate(dateStr string) time.Time {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		panic(err)
	}
	return date
}

package utils

import (
	"fmt"
	telegram "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
	"tutor/usagetracker"

	conf "tutor/config"
)

func messageText(message *telegram.Message) string {
	if message.Text == "" {
		return ""
	}

	messageTxt := message.Text
	entities := *message.Entities
	for _, entity := range entities {
		if entity.Type == "bot_command" {
			messageTxt = strings.ReplaceAll(messageTxt, message.Text[entity.Offset:entity.Offset+entity.Length], "")
		}
	}

	return strings.TrimSpace(messageTxt)
}

func isUserInGroup(bot *telegram.BotAPI, chatID int64, userID int) (bool, error) {
	member, err := bot.GetChatMember(telegram.ChatConfigWithUser{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		if err.Error() == "User not found" {
			return false, nil
		}
		return false, err
	}
	return member.IsMember(), nil
}

func getThreadID(message *telegram.Message) int64 {
	if message.MessageID != 0 {
		return int64(message.MessageID)
	}
	return 0
}

func getStreamCutoffValues(message *telegram.Message, content string) int {
	if isGroupChat(message.Chat) {
		if len(content) > 1000 {
			return 180
		} else if len(content) > 200 {
			return 120
		} else if len(content) > 50 {
			return 90
		}
		return 50
	} else {
		if len(content) > 1000 {
			return 90
		} else if len(content) > 200 {
			return 45
		} else if len(content) > 50 {
			return 25
		}
		return 15
	}
}

func isGroupChat(chat *telegram.Chat) bool {
	return chat.IsGroup() || chat.IsSuperGroup()
}

func splitIntoChunks(text string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(text); i += chunkSize {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
	}
	return chunks
}

func wrapWithIndicator(bot *telegram.BotAPI, chatID int64, action string, coroutine func() error) error {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	done := make(chan error)
	go func() {
		done <- coroutine()
	}()

	for {
		select {
		case <-ticker.C:
			bot.Send(telegram.NewChatAction(chatID, action))
		case err := <-done:
			return err
		}
	}
}

func editMessageWithRetry(bot *telegram.BotAPI, chatID int64, messageID int, text string, markdown bool) error {
	msg := telegram.NewEditMessageText(chatID, messageID, text)
	if markdown {
		msg.ParseMode = telegram.ModeMarkdown
	}

	_, err := bot.Send(msg)
	if err != nil {
		if strings.Contains(err.Error(), "Message is not modified") {
			return nil
		}

		msg.ParseMode = ""
		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Failed to edit message: %v", err)
			return err
		}
	}

	return nil
}

func errorHandler(err error) {
	log.Printf("Exception while handling an update: %v", err)
}

func isAllowed(config conf.Config, update *telegram.Update, bot *telegram.BotAPI, isInline bool) (bool, error) {
	if config.AllowedUserIDs == "*" {
		return true, nil
	}

	var userID int
	if isInline {
		userID = update.InlineQuery.From.ID
	} else {
		userID = update.Message.From.ID
	}

	if IsAdmin(config, userID) {
		return true, nil
	}

	allowedUserIDs := strings.Split(config.AllowedUserIDs, ",")
	for _, id := range allowedUserIDs {
		if id == fmt.Sprint(userID) {
			return true, nil
		}
	}

	if !isInline && isGroupChat(update.Message.Chat) {
		adminUserIDs := strings.Split(config.AdminUserIDs, ",")
		for _, id := range append(allowedUserIDs, adminUserIDs...) {
			if id == "" {
				continue
			}
			isMember, err := isUserInGroup(bot, update.Message.Chat.ID, userID)
			if err != nil {
				return false, err
			}
			if isMember {
				log.Printf("%d is a member. Allowing group chat message...", userID)
				return true, nil
			}
		}
		log.Printf("Group chat messages from user %s (id: %d) are not allowed", update.Message.From.UserName, userID)
	}

	return false, nil
}

func IsAdmin(config conf.Config, userID int) bool {
	if config.AdminUserIDs == "-" {
		log.Println("No admin user defined.")
		return false
	}

	adminUserIDs := strings.Split(config.AdminUserIDs, ",")
	for _, id := range adminUserIDs {
		if id == fmt.Sprint(userID) {
			return true
		}
	}

	return false
}

func GetUserBudget(cfg conf.Config, userID int) float64 {
	if IsAdmin(cfg, userID) || cfg.UserBudgets == "*" {
		return math.Inf(1)
	}

	userBudgets := strings.Split(cfg.UserBudgets, ",")
	allowedUserIDs := strings.Split(cfg.AllowedUserIDs, ",")

	for idx, id := range allowedUserIDs {
		if id == fmt.Sprint(userID) {
			if idx >= len(userBudgets) {
				log.Printf("No budget set for user id: %d. Budget list shorter than user list.", userID)
				return 0.0
			}
			budget, err := strconv.ParseFloat(userBudgets[idx], 64)
			if err != nil {
				log.Printf("Error parsing budget for user id: %d", userID)
				return 0.0
			}
			return budget
		}
	}

	return 0.0
}

func GetRemainingBudget(cfg conf.Config, usage map[string]*usagetracker.UsageTracker, update *telegram.Update, isInline bool) float64 {
	budgetCostMap := map[string]string{
		"monthly":  "cost_month",
		"daily":    "cost_today",
		"all-time": "cost_all_time",
	}

	var userID string
	if isInline {
		userID = fmt.Sprintf("%d", update.InlineQuery.From.ID)
	} else {
		userID = fmt.Sprintf("%d", update.Message.From.ID)
	}

	if _, ok := usage[userID]; !ok {
		usage[userID] = usagetracker.NewUsageTracker(update.Message.From.ID, update.Message.From.UserName)
	}

	userBudget := GetUserBudget(cfg, update.Message.From.ID)
	budgetPeriod := cfg.BudgetPeriod
	cost := usage[userID].GetCurrentCost(budgetCostMap[budgetPeriod])

	return userBudget - cost
}

// IsWithinBudget checks if the user reached their usage limit.
func IsWithinBudget(cfg conf.Config, usage map[string]*usagetracker.UsageTracker, update *telegram.Update, isInline bool) bool {
	var _ string
	if isInline {
		_ = fmt.Sprintf("%d", update.InlineQuery.From.ID)
	} else {
		_ = fmt.Sprintf("%d", update.Message.From.ID)
	}

	remainingBudget := GetRemainingBudget(cfg, usage, update, isInline)
	return remainingBudget > 0
}

func AddChatRequestToUsageTracker(usage map[string]*usagetracker.UsageTracker, cfg conf.Config, userID int, usedTokens int) {
	userIDStr := fmt.Sprintf("%d", userID)
	if _, ok := usage[userIDStr]; !ok {
		usage[userIDStr] = usagetracker.NewUsageTracker(userID, fmt.Sprintf("User %d", userID))
	}

	userTracker := usage[userIDStr]
	userTracker.AddChatTokens(usedTokens, cfg.TokenPrice)

	if !strings.Contains(cfg.AllowedUserIDs, userIDStr) {
		if _, ok := usage["guests"]; !ok {
			usage["guests"] = usagetracker.NewUsageTracker(-1, "Guests")
		}
		guestTracker := usage["guests"]
		guestTracker.AddChatTokens(usedTokens, cfg.TokenPrice)
	}
}

func getReplyToMessageID(config conf.Config, message *telegram.Message) int {
	if config.EnableQuoting || isGroupChat(message.Chat) {
		return message.MessageID
	}
	return 0
}

package service

import (
	"errors"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TelegramService interface {
	SendMessage(userID, message string) error
	SendMessageToChat(chatID int64, message string) error
}

type telegramService struct {
	bot         *tgbotapi.BotAPI
	userService UserService
	logService  LogService
}

func NewTelegramService(botToken string, userService UserService, logService LogService) (TelegramService, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, errors.New("failed to initialize Telegram bot: " + err.Error())
	}

	return &telegramService{
		bot:         bot,
		userService: userService,
		logService:  logService,
	}, nil
}

func (s *telegramService) SendMessage(userID, message string) error {
	if message == "" {
		return errors.New("message cannot be empty")
	}

	user, err := s.userService.GetUser(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	chatID, err := strconv.ParseInt(user.TelegramID, 10, 64)
	if err != nil {
		return errors.New("invalid Telegram ID format: " + err.Error())
	}

	if chatID == 0 {
		return errors.New("user does not have a linked Telegram ID")
	}

	msg := tgbotapi.NewMessage(chatID, message)
	_, err = s.bot.Send(msg)
	if err != nil {
		return errors.New("failed to send Telegram message: " + err.Error())
	}

	metadata := map[string]interface{}{
		"user_id":     userID,
		"telegram_id": user.TelegramID,
		"message":     message,
	}
	if err := s.logService.LogAction(primitive.ObjectID{}, "SendTelegramMessage", "Message sent to user", "", metadata); err != nil {
		return err
	}

	return nil
}

func (s *telegramService) SendMessageToChat(chatID int64, message string) error {
	if message == "" {
		return errors.New("message cannot be empty")
	}

	if chatID == 0 {
		return errors.New("invalid chat ID")
	}

	msg := tgbotapi.NewMessage(chatID, message)
	_, err := s.bot.Send(msg)
	if err != nil {
		return errors.New("failed to send Telegram message: " + err.Error())
	}

	metadata := map[string]interface{}{
		"chat_id": chatID,
		"message": message,
	}
	if err := s.logService.LogAction(primitive.ObjectID{}, "SendTelegramMessageToChat", "Message sent to chat", "", metadata); err != nil {
		return err
	}

	return nil
}

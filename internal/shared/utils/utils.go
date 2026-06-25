package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/qRe0/afterparty-bot/internal/configs"
	"github.com/qRe0/afterparty-bot/internal/models"
)

const (
	vipTicketTypeTemplate = "вип%d"
	formattedFIOTemplate  = "%s %s %s"
	formattedFITemplate   = "%s %s"
)

func ShowOptions(chatID int64, bot *tgbotapi.BotAPI, userName string, cfg configs.AllowList) {
	msg := tgbotapi.NewMessage(chatID, "Выберите опцию:")

	checker := UserInList(userName, cfg.AllowedCheckers)
	seller := UserInList(userName, cfg.AllowedSellers)

	var keyboard tgbotapi.ReplyKeyboardMarkup
	var row []tgbotapi.KeyboardButton

	if checker {
		row = append(row, tgbotapi.NewKeyboardButton("Отметить вход"))
	}

	if seller {
		row = append(row, tgbotapi.NewKeyboardButton("Продать билет"))
	}

	if len(row) > 0 {
		keyboard = tgbotapi.NewReplyKeyboard(row)
	}

	msg.ReplyMarkup = keyboard
	_, _ = bot.Send(msg)
}

func ResponseMapper(resp *models.TicketResponse, cfg configs.LacesColors) string {
	successEmoji := "ДА ✅✅✅"
	failEmoji := "НЕТ ❌❌❌"

	var laceColor string
	ticketType := strings.ToLower(resp.TicketType)
	switch {
	case ticketType == "орг":
		laceColor = cfg.Org
	case strings.HasPrefix(ticketType, "вип"):
		laceColor = cfg.VIP
	case ticketType == "базовый":
		laceColor = cfg.Base
	default:
		return "Неизвестный тип билета"
	}

	controlStatus := failEmoji
	if resp.PassedControlZone {
		controlStatus = successEmoji
	}

	return fmt.Sprintf("Номер билета: %s,\nФИО: %s,\nТип браслета: %s,\nЦвет браслета: %s,\nПрошел контроль? - %s",
		resp.Id, resp.Name, resp.TicketType, laceColor, controlStatus)
}

func ValidateTicketType(ticketType string, cfg configs.SalesOptions) (string, bool) {
	ticketType = strings.ToLower(ticketType)
	allowedTicketTypes := make([]string, 0)
	allowedTicketTypes = append(allowedTicketTypes, "базовый")
	for i := 0; i < cfg.VIPTablesCount; i++ {
		allowedTicketTypes = append(allowedTicketTypes, fmt.Sprintf(vipTicketTypeTemplate, i+1))
	}

	allowed := false
	for _, allowedTicketType := range allowedTicketTypes {
		if ticketType == allowedTicketType {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", false
	}

	return ticketType, true
}

func ParseTicketPrice(input, username string, cfg configs.AllowList) (int, error) {
	if input == "" {
		return 0, fmt.Errorf("ticket price is not specified")
	}

	re := regexp.MustCompile(`\d+`)
	match := re.FindString(input)
	if match == "" {
		return 0, fmt.Errorf("failed to find digit in string: %s", input)
	}

	value, err := strconv.Atoi(match)
	if err != nil {
		return 0, fmt.Errorf("failed to parse string to int %q: %v", match, err)
	}

	switch value {
	case 15:
		if UserInList(username, cfg.SSSellers) {
			return 15, nil
		} else {
			return -1, fmt.Errorf("failed to parse ticket price. wrong value")
		}
	case 17:
		return 17, nil
	case 20:
		return 20, nil
	case 22:
		return 22, nil
	case 25:
		return 25, nil
	case 30:
		return 30, nil
	case 13:
		return 13, nil
	default:
		return -1, fmt.Errorf("failed to parse ticket price. wrong value")
	}
}

func FormatFIO(fio string) (string, error) {
	parts := strings.Fields(fio)

	if len(parts) < 2 {
		return "", fmt.Errorf("failed to parse client's full name")
	}
	if len(parts) == 2 {
		return fmt.Sprintf(formattedFITemplate, parts[0], parts[1]), nil
	}

	return fmt.Sprintf(formattedFIOTemplate, parts[0], parts[1], parts[2]), nil
}

func CheckRepost(msg string) bool {
	keyWord := strings.ToLower(msg)

	switch keyWord {
	case "да":
		return true
	case "нет":
		return false
	default:
		return false
	}
}

func convertStringsToDates(dates []string) ([]time.Time, error) {
	if len(dates) == 0 {
		return nil, fmt.Errorf("dates slice can't be empty")
	}

	var result []time.Time
	for _, dateStr := range dates {
		t, err := time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, fmt.Errorf("failes to parse date %q: %w", dateStr, err)
		}
		result = append(result, t)
	}

	return result, nil
}

// prices = {20,17,25,22,30} ->
// prices[0] - цена без репоста до повышения
// prices[1] - цена с репостом до повышения
// prices[2] - цена без репоста после повышения
// prices[3] - цена с репостом после повышения
// prices[4] - цена одного ВИП-билета

// dates = {xxx} ->
// dates[0] - дата повышения
// dates[1] - дата конца продаж

func CalculateActualTicketPrice(timeNow time.Time, cfg configs.SalesOptions, client models.ClientData) int {
	dates, err := convertStringsToDates(cfg.Dates)
	if err != nil {
		return -1
	}

	ticketType := strings.ToLower(client.TicketType)
	switch {
	case ticketType == "базовый":
		if timeNow.Before(dates[0]) {
			if client.RepostExists {
				return cfg.Prices[1]
			} else {
				return cfg.Prices[0]
			}
		} else if timeNow.After(dates[0]) {
			if client.RepostExists {
				return cfg.Prices[3]
			} else {
				return cfg.Prices[2]
			}
		}
	case strings.HasPrefix(ticketType, "вип"):
		return cfg.Prices[4]
	}

	return -1
}

func GetSurnameLowercase(surname string) string {
	parts := strings.Split(surname, " ")

	if len(parts) < 2 {
		return ""
	}

	formattedSurname := strings.ToLower(parts[0])
	formattedSurname = strings.Replace(formattedSurname, "ё", "е", -1)
	return formattedSurname
}

func UserInList(userName string, list map[string]bool) bool {
	return list[userName]
}

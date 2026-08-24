package outbrain

import (
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

func validOpaque(value string, maximum int) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func validPathID(value string) bool {
	return validOpaque(value, 256) && !strings.ContainsAny(value, "/?#% ,")
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validOptionalText(value *string, maximum int) bool {
	return value == nil || validText(*value, maximum)
}

func validPositive(value *float64) bool {
	return value == nil || (*value > 0 && *value <= 1_000_000_000)
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func validOptionalDate(value *string) bool { return value == nil || validDate(*value) }

func validDateWindow(from, to string) bool {
	if !validDate(from) || !validDate(to) {
		return false
	}
	start, _ := time.Parse("2006-01-02", from)
	end, _ := time.Parse("2006-01-02", to)
	return !end.Before(start)
}

func validDestinationURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && len(value) <= 4096
}

func validPage(limit, offset, maximum int) bool {
	return limit >= 0 && offset >= 0 && (limit == 0 || limit <= maximum)
}

func validFilter(value string) bool { return value == "" || validOpaque(value, 512) }

func validTimezone(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 9 || !strings.HasPrefix(value, "GMT") || (value[3] != '+' && value[3] != '-') || value[6] != ':' {
		return false
	}
	_, first := time.Parse("15:04", value[4:])
	return first == nil
}

func validIDs(values []string) bool {
	for _, value := range values {
		if !validPathID(value) {
			return false
		}
	}
	return true
}

func validStringList(values []string, maximumItems, maximumLength int) bool {
	if len(values) > maximumItems {
		return false
	}
	for _, value := range values {
		if !validText(value, maximumLength) {
			return false
		}
	}
	return true
}

func validBudgetType(value BudgetType) bool {
	return value == BudgetCampaign || value == BudgetMonthly || value == BudgetDaily
}

func validPacingType(value PacingType) bool {
	return value == PacingSpendASAP || value == PacingAutomatic || value == PacingDailyTarget
}

func validBudgetMinimum(input CreateBudgetRequest) bool {
	if input.Amount <= 0 {
		return false
	}
	switch input.Type {
	case BudgetDaily:
		return input.Amount >= 25
	case BudgetMonthly:
		return input.Amount >= 750
	case BudgetCampaign:
		if !validDateWindow(input.StartDate, input.EndDate) {
			return false
		}
		start, _ := time.Parse("2006-01-02", input.StartDate)
		end, _ := time.Parse("2006-01-02", input.EndDate)
		days := int(end.Sub(start)/(24*time.Hour)) + 1
		return input.Amount >= float64(25*days)
	default:
		return false
	}
}

func validTargeting(value CampaignTargeting) bool {
	return validStringList(value.Platform, 16, 64) && validStringList(value.Locations, 1000, 256) &&
		validStringList(value.OperatingSystems, 64, 128) && validStringList(value.Browsers, 64, 128)
}

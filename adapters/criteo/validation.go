package criteo

import (
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validOpaque(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	nonzero := false
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
		nonzero = nonzero || character != '0'
	}
	return nonzero
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || (utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n"))
}

func validPositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validOptionalPositive(value *float64) bool {
	return value == nil || validPositive(*value)
}

func validIDs(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validStrings[T ~string](values []T, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[T]struct{}, len(values))
	for _, value := range values {
		if !validText(string(value), 128) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validDateTime(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validOptionalDateTime(value *string) bool {
	return value == nil || validDateTime(*value)
}

func parseReportDate(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func validCampaignGoal(value CampaignGoal) bool {
	return value == GoalAcquisition || value == GoalRetention
}

func validSpendLimit(value CreateCampaignSpendLimit) bool {
	switch value.Type {
	case SpendLimitCapped:
		return validOptionalPositive(value.Amount) && value.Amount != nil &&
			(value.Renewal == RenewalDaily || value.Renewal == RenewalMonthly || value.Renewal == RenewalLifetime)
	case SpendLimitUncapped:
		return value.Amount == nil && (value.Renewal == "" || value.Renewal == RenewalUndefined)
	default:
		return false
	}
}

func validMediaType(value MediaType) bool { return value == MediaDisplay || value == MediaVideo }

func validObjective(value Objective) bool {
	switch value {
	case ObjectiveCustomAction, ObjectiveClicks, ObjectiveConversions, ObjectiveDisplays,
		ObjectiveAppPromotion, ObjectiveRevenue, ObjectiveStoreConversions, ObjectiveValue,
		ObjectiveReach, ObjectiveVisits, ObjectiveVideoViews:
		return true
	default:
		return false
	}
}

func validCostController(value CostController) bool {
	switch value {
	case CostCOS, CostMaxCPC, CostCPI, CostCPM, CostCPO, CostCPSV, CostCPV, CostDailyBudget, CostTargetCPM:
		return true
	default:
		return false
	}
}

func validBudget(value CreateAdSetBudget) bool {
	if !validOptionalPositive(value.Amount) {
		return false
	}
	if value.DeliverySmoothing != "" && value.DeliverySmoothing != DeliveryAccelerated && value.DeliverySmoothing != DeliveryStandard {
		return false
	}
	switch value.Strategy {
	case BudgetCapped:
		if value.Amount == nil || (value.Renewal != BudgetDaily && value.Renewal != BudgetMonthly && value.Renewal != BudgetLifetime && value.Renewal != BudgetWeekly) {
			return false
		}
		return value.Renewal != BudgetWeekly || validDeliveryWeek(value.DeliveryWeek)
	case BudgetUncapped:
		return value.Amount == nil && (value.Renewal == "" || value.Renewal == BudgetUndefined) &&
			(value.DeliveryWeek == "" || value.DeliveryWeek == WeekUndefined)
	default:
		return false
	}
}

func validDeliveryWeek(value DeliveryWeek) bool {
	switch value {
	case WeekMondayToSunday, WeekTuesdayToMonday, WeekWednesdayToTuesday, WeekThursdayToWednesday,
		WeekFridayToThursday, WeekSaturdayToFriday, WeekSundayToSaturday:
		return true
	default:
		return false
	}
}

func validFrequency(value FrequencyCapping) bool {
	if value.Frequency == "" && value.MaximumImpressions == 0 {
		return true
	}
	if value.MaximumImpressions <= 0 {
		return false
	}
	switch value.Frequency {
	case FrequencyHourly, FrequencyDaily, FrequencyLifetime, FrequencyAdvanced:
		return true
	default:
		return false
	}
}

func validTargetingRule(value *TargetingRule) bool {
	if value == nil {
		return true
	}
	if value.Operand != OperandIn && value.Operand != OperandNotIn {
		return false
	}
	return len(value.Values) > 0 && validStrings(value.Values, 1000)
}

func validDeliveryLimitations(value DeliveryLimitations) bool {
	for _, device := range value.Devices {
		if device != DeviceOther && device != DeviceDesktop && device != DeviceMobile && device != DeviceTablet {
			return false
		}
	}
	for _, environment := range value.Environments {
		if environment != EnvironmentWeb && environment != EnvironmentInApp {
			return false
		}
	}
	for _, operatingSystem := range value.OperatingSystems {
		if operatingSystem != OSAndroid && operatingSystem != OSIOS && operatingSystem != OSUnknown {
			return false
		}
	}
	return validStrings(value.Devices, 16) && validStrings(value.Environments, 16) && validStrings(value.OperatingSystems, 16)
}

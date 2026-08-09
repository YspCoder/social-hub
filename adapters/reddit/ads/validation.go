package ads

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validUserAgent(value string) bool {
	return len(value) >= 12 && len(value) <= 256 && strings.Contains(value, ":") &&
		strings.Contains(value, "(by /u/") && !strings.ContainsAny(value, "\r\n")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validAdAccountID(value string) bool {
	return (strings.HasPrefix(value, "t2_") || strings.HasPrefix(value, "a2_")) && validPrefixedID(value, 128)
}

func validPixelID(value string) bool {
	return (strings.HasPrefix(value, "t2_") || strings.HasPrefix(value, "a2_") || strings.HasPrefix(value, "p2_")) && validPrefixedID(value, 128)
}

func validPostID(value string) bool {
	return strings.HasPrefix(value, "t3_") && validPrefixedID(value, 128)
}

func validPrefixedID(value string, maximum int) bool {
	if len(value) < 4 || len(value) > maximum {
		return false
	}
	for index, character := range value {
		if index == 2 && character == '_' {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'z' {
				if character < 'A' || character > 'Z' {
					return false
				}
			}
		}
	}
	return true
}

func validResourceID(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validList(input ListRequest) bool {
	return (input.Cursor == "" || validOpaque(input.Cursor, 16384)) && input.PageSize >= 0 && input.PageSize <= 1000
}

func validMutationStatus(value ConfiguredStatus) bool {
	return value == StatusActive || value == StatusPaused
}

func validObjective(value CampaignObjective) bool {
	switch value {
	case ObjectiveAppInstalls, ObjectiveCatalogSales, ObjectiveClicks, ObjectiveConversions,
		ObjectiveImpressions, ObjectiveLeadGeneration, ObjectiveVideoViewableImpressions:
		return true
	default:
		return false
	}
}

func validGoalType(value GoalType) bool {
	return value == GoalDailySpend || value == GoalLifetimeSpend
}

func validBidStrategy(value BidStrategy) bool {
	return value == BidStrategyBidless || value == BidStrategyManual || value == BidStrategyMaximizeVolume || value == BidStrategyTargetCPX
}

func validCampaignBidStrategy(value BidStrategy) bool {
	return value == BidStrategyBidless || value == BidStrategyMaximizeVolume || value == BidStrategyTargetCPX
}

func validBidType(value BidType) bool {
	return value == BidTypeCPC || value == BidTypeCPM || value == BidTypeCPV || value == BidTypeCPV6
}

func validClickURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil &&
		len(value) <= 5000 && !strings.ContainsAny(value, "\r\n")
}

func hourAligned(value time.Time) bool {
	return !value.IsZero() && value.Equal(value.Truncate(time.Hour))
}

func validReportField(value ReportField) bool {
	return validEnumToken(string(value), 128)
}

func validReportBreakdown(value ReportBreakdown) bool {
	switch value {
	case BreakdownAdAccountID, BreakdownCampaignID, BreakdownAdGroupID, BreakdownAdID,
		BreakdownDate, BreakdownHour, BreakdownCountry, BreakdownRegion, BreakdownPlacement:
		return true
	default:
		return validEnumToken(string(value), 64)
	}
}

func validEnumToken(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func containsBreakdown(values []ReportBreakdown, target ReportBreakdown) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

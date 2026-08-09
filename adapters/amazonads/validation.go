package amazonads

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validID(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	nonZero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonZero = nonZero || value[index] != '0'
	}
	return nonZero
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
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && len(value) <= maximum &&
		utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validState(value State) bool { return value == StateEnabled || value == StatePaused }

func validStates(values []State) bool {
	for _, value := range values {
		if !validState(value) && value != StateProposed {
			return false
		}
	}
	return true
}

func validTargetingType(value TargetingType) bool {
	return value == TargetingAuto || value == TargetingManual
}

func validMatchType(value MatchType) bool {
	return value == MatchBroad || value == MatchPhrase || value == MatchExact
}

func validBiddingStrategy(value BiddingStrategy) bool {
	switch value {
	case BiddingLegacyForSales, BiddingAutoForSales, BiddingManual, BiddingRuleBased:
		return true
	default:
		return false
	}
}

func validDynamicBidding(value DynamicBidding) bool {
	if !validBiddingStrategy(value.Strategy) || len(value.PlacementBidding) > 3 {
		return false
	}
	seen := make(map[string]struct{}, len(value.PlacementBidding))
	for _, adjustment := range value.PlacementBidding {
		switch adjustment.Placement {
		case "PLACEMENT_TOP", "PLACEMENT_PRODUCT_PAGE", "PLACEMENT_REST_OF_SEARCH":
		default:
			return false
		}
		if adjustment.Percentage < 0 || adjustment.Percentage > 900 {
			return false
		}
		if _, found := seen[adjustment.Placement]; found {
			return false
		}
		seen[adjustment.Placement] = struct{}{}
	}
	return true
}

func validDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func validDecimal(value string, positive bool) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	dot, nonZero := false, false
	for index := range value {
		character := value[index]
		if character == '.' {
			if dot || index == 0 || index == len(value)-1 {
				return false
			}
			dot = true
			continue
		}
		if character < '0' || character > '9' {
			return false
		}
		nonZero = nonZero || character != '0'
	}
	return !positive || nonZero
}

func validList(maxResults int, nextToken string) bool {
	return maxResults >= 0 && maxResults <= 1000 && (nextToken == "" || validOpaque(nextToken, 16384))
}

func validIDs(values []string) bool {
	if len(values) > 1000 {
		return false
	}
	for _, value := range values {
		if !validID(value) {
			return false
		}
	}
	return true
}

func validPathID(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '-' && character != '_' && (character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validASIN(value string) bool {
	if len(value) != 10 {
		return false
	}
	for index := range value {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}

func requireMutationID(operation, expected, actual string) error {
	if !validID(actual) || expected != "" && actual != expected {
		return platformContractError(operation, "Amazon Ads returned a missing or mismatched resource ID")
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'A' && (value[0] < 'a' || value[0] > 'z') || value[0] > 'Z' && value[0] < 'a' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character != '_' && (character < '0' || character > '9') && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validIdentifiers(values []string, minimum, maximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
	}
	return true
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

package ads

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validID(value string) bool {
	if value == "" || len(value) > 18 {
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

func validOptionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func validStatus(value EntityStatus) bool {
	switch value {
	case StatusActive, StatusPaused, StatusArchived, StatusDraft, StatusDeletedDraft:
		return true
	default:
		return false
	}
}

func validMutationStatus(value EntityStatus) bool {
	return value == StatusActive || value == StatusPaused
}

func validStatuses(values []EntityStatus) bool {
	for _, value := range values {
		if !validStatus(value) {
			return false
		}
	}
	return true
}

func validObjective(value ObjectiveType) bool {
	switch value {
	case ObjectiveAwareness, ObjectiveConsideration, ObjectiveWebConversion, ObjectiveCatalogSales,
		ObjectiveVideoCompletion, ObjectiveAppInstall, ObjectiveSales, ObjectiveLeads, ObjectiveCTVConsideration:
		return true
	default:
		return false
	}
}

func validBillableEvent(value BillableEvent) bool {
	return value == BillableClickthrough || value == BillableImpression || value == BillableVideoView50MRC
}

func validBudgetType(value BudgetType) bool {
	return value == BudgetDaily || value == BudgetLifetime || value == BudgetCBOAdGroup
}

func validBidStrategy(value BidStrategyType) bool {
	return value == "" || value == BidAutomatic || value == BidMaximum || value == BidTargetAverage
}

func validPacing(value PacingDeliveryType) bool {
	return value == PacingStandard || value == PacingAccelerated
}

func validPlacement(value PlacementGroup) bool {
	return value == PlacementAll || value == PlacementSearch || value == PlacementBrowse || value == PlacementOther
}

func validCreativeType(value CreativeType) bool {
	switch value {
	case CreativeRegular, CreativeVideo, CreativeShopping, CreativeCarousel, CreativeMaxVideo,
		CreativeShopThePin, CreativeCollection, CreativeIdea, CreativeShowcase, CreativeQuiz,
		CreativeCollage, CreativeMaxWidthRegularCollection, CreativeMaxWidthVideoCollection, CreativeApp:
		return true
	default:
		return false
	}
}

func validPage(cursor string, maxResults int) bool {
	return (cursor == "" || validOpaque(cursor, 16384)) && maxResults >= 0 && maxResults <= 250
}

func validIDs(values []string) bool {
	if len(values) > 250 {
		return false
	}
	for _, value := range values {
		if !validID(value) {
			return false
		}
	}
	return true
}

func validSchedule(start, end int64) bool {
	return start >= 0 && end >= 0 && (start == 0 || end == 0 || end > start)
}

func validUpdateSchedule(start, end *int64) bool {
	if start != nil && *start < 0 || end != nil && *end < -1 {
		return false
	}
	return start == nil || end == nil || *end == -1 || *start == 0 || *end == 0 || *end > *start
}

func validCampaignBudget(cbo bool, daily, lifetime int64) bool {
	if daily < 0 || lifetime < 0 || daily > 0 && lifetime > 0 {
		return false
	}
	if cbo {
		return daily > 0 || lifetime > 0
	}
	return daily == 0 && lifetime == 0
}

func validTargeting(targeting TargetingSpec) bool {
	if len(targeting) == 0 || len(targeting) > 32 {
		return false
	}
	hasLocation := false
	for key, values := range targeting {
		if !validUpperIdentifier(key) || len(values) == 0 || len(values) > 500 {
			return false
		}
		if key == "LOCATION" || key == "GEO" {
			hasLocation = true
		}
		for _, value := range values {
			if !validOpaque(value, 512) {
				return false
			}
		}
	}
	return hasLocation
}

func validJSONMap(value map[string]any) bool {
	if value == nil {
		return true
	}
	encoded, err := json.Marshal(value)
	return err == nil && len(encoded) <= 64<<10
}

func validHTTPURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validDate(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func dateSpan(start, end string) (int, bool) {
	if !validDate(start) || !validDate(end) {
		return 0, false
	}
	from, _ := time.Parse("2006-01-02", start)
	to, _ := time.Parse("2006-01-02", end)
	if to.Before(from) {
		return 0, false
	}
	return int(to.Sub(from)/(24*time.Hour)) + 1, true
}

func validUpperIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character != '_' && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validColumns(values []string) bool {
	if len(values) == 0 || len(values) > 100 {
		return false
	}
	for _, value := range values {
		if !validUpperIdentifier(value) {
			return false
		}
	}
	return true
}

func validAttributionWindow(value int) bool {
	switch value {
	case 0, 1, 7, 14, 30, 60:
		return true
	default:
		return false
	}
}

func validOAuthScope(value string) bool {
	return value == adsReadScope || value == adsWriteScope
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

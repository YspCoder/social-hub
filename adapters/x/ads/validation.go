package ads

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validAdsID(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	for index := range value {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validTweetID(value string) bool {
	if len(value) == 0 || len(value) > 20 || value[0] == '0' {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
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

func validList(cursor string, count int) bool {
	return (cursor == "" || validOpaque(cursor, 16384)) && count >= 0 && count <= 1000
}

func validUniqueAdsIDs(values []string, maximum int) bool {
	if len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validAdsID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validUniqueTweetIDs(values []string, maximum int) bool {
	if len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validTweetID(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func hourAligned(value time.Time) bool {
	return !value.IsZero() && value.Equal(value.Truncate(time.Hour))
}

func formatInt(value int) string { return strconv.Itoa(value) }

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }

func validMutationStatus(value EntityStatus) bool {
	return value == StatusActive || value == StatusPaused
}

func validObjective(value Objective) bool {
	switch value {
	case ObjectiveAppEngagements, ObjectiveAppInstalls, ObjectiveReach, ObjectiveFollowers,
		ObjectiveEngagements, ObjectiveVideoViews, ObjectivePrerollViews, ObjectiveWebsiteClicks:
		return true
	default:
		return false
	}
}

func validProductType(value ProductType) bool {
	return value == ProductMedia || value == ProductPromotedAccount || value == ProductPromotedTweets
}

func validPlacements(values []Placement) bool {
	if len(values) == 0 || len(values) > 12 {
		return false
	}
	seen := make(map[Placement]struct{}, len(values))
	for _, value := range values {
		switch value {
		case PlacementAllOnTwitter, PlacementPublisherNetwork, PlacementTapBanner, PlacementTapFull,
			PlacementTapFullLandscape, PlacementTapNative, PlacementTapMRect, PlacementTwitterProfile,
			PlacementTwitterReplies, PlacementTwitterSearch, PlacementTwitterTimeline:
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validBidStrategy(value BidStrategy) bool {
	return value == BidStrategyAuto || value == BidStrategyMax || value == BidStrategyTarget
}

func validAnalyticsEntity(value AnalyticsEntity) bool {
	switch value {
	case AnalyticsAccount, AnalyticsCampaign, AnalyticsFundingInstrument, AnalyticsLineItem,
		AnalyticsPromotedAccount, AnalyticsPromotedTweet:
		return true
	default:
		return false
	}
}

func validGranularity(value Granularity) bool {
	return value == GranularityDay || value == GranularityHour || value == GranularityTotal
}

func validAnalyticsPlacement(value AnalyticsPlacement) bool {
	return value == AnalyticsPlacementAllOnTwitter || value == AnalyticsPlacementSpotlight || value == AnalyticsPlacementTrend
}

func validMetricGroups(values []MetricGroup) bool {
	if len(values) == 0 || len(values) > 6 {
		return false
	}
	seen := make(map[MetricGroup]struct{}, len(values))
	for _, value := range values {
		switch value {
		case MetricGroupBilling, MetricGroupEngagement, MetricGroupLifetimeMobileConversion,
			MetricGroupMobileConversion, MetricGroupVideo, MetricGroupWebConversion:
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

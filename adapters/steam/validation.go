package steam

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxNewsCount     = 100
	maxNewsLength    = 64 << 10
	maxNewsFeedCount = 20
	maxNewsFeedBytes = 2048
)

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validWebAPIKey(value string) bool {
	if !validOpaque(value, 4096) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func canonicalSteamID(value SteamID) (string, bool) {
	text := string(value)
	if text == "" || len(text) > 20 {
		return "", false
	}
	for _, character := range text {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil || parsed == 0 {
		return "", false
	}
	return strconv.FormatUint(parsed, 10), true
}

func validatePlayerSummaries(input GetPlayerSummariesRequest) error {
	if len(input.SteamIDs) == 0 || len(input.SteamIDs) > 100 {
		return invalidArgument("get_player_summaries", "steam_ids must contain between 1 and 100 values")
	}
	seen := make(map[string]struct{}, len(input.SteamIDs))
	for _, steamID := range input.SteamIDs {
		canonical, valid := canonicalSteamID(steamID)
		if !valid {
			return invalidArgument("get_player_summaries", "each SteamID must be a positive uint64 decimal string of at most 20 digits")
		}
		if _, exists := seen[canonical]; exists {
			return invalidArgument("get_player_summaries", "steam_ids must not contain duplicates")
		}
		seen[canonical] = struct{}{}
	}
	return nil
}

func validateNewsRequest(input GetNewsForAppRequest) error {
	if input.AppID == 0 {
		return invalidArgument("get_news_for_app", "app_id must be a positive uint32 value")
	}
	if input.Count > maxNewsCount {
		return invalidArgument("get_news_for_app", "count exceeds the adapter safety limit of 100")
	}
	if input.MaxLength > maxNewsLength {
		return invalidArgument("get_news_for_app", "max_length exceeds the adapter safety limit of 65536")
	}
	if input.EndDate != nil && (input.EndDate.IsZero() || input.EndDate.Unix() <= 0 || input.EndDate.Unix() > int64(^uint32(0))) {
		return invalidArgument("get_news_for_app", "end_date must fit Steam's positive uint32 Unix timestamp")
	}
	if err := validateNewsValues("feeds", input.Feeds); err != nil {
		return err
	}
	return validateNewsValues("tags", input.Tags)
}

func validateNewsValues(name string, values []string) error {
	if len(values) > maxNewsFeedCount {
		return invalidArgument("get_news_for_app", name+" exceeds the adapter safety limit of 20")
	}
	seen, total := make(map[string]struct{}, len(values)), 0
	for _, value := range values {
		if !validOpaque(value, 256) || strings.Contains(value, ",") {
			return invalidArgument("get_news_for_app", "each "+name+" value must be non-empty and contain no commas")
		}
		if _, exists := seen[value]; exists {
			return invalidArgument("get_news_for_app", name+" must not contain duplicates")
		}
		seen[value] = struct{}{}
		total += len(value)
	}
	if total > maxNewsFeedBytes {
		return invalidArgument("get_news_for_app", "combined "+name+" values exceed the adapter safety limit")
	}
	return nil
}

func validNewsValues(values []string) bool {
	if len(values) > maxNewsFeedCount {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	total := 0
	for _, value := range values {
		if !validOpaque(value, 256) || strings.Contains(value, ",") {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
		total += len(value)
	}
	return total <= maxNewsFeedBytes
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Steam Web API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Steam Web API operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Steam Web API operation")
	}
	return nil
}

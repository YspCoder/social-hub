package applovinads

import (
	"math/big"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumListSize       = 100
	maximumUploadFiles    = 40
	maximumUploadFileSize = int64(1 << 30)
	maximumUploadTotal    = int64(10 << 30)
)

var (
	numericIDPattern  = regexp.MustCompile(`^[1-9][0-9]{0,63}$`)
	countryPattern    = regexp.MustCompile(`^[A-Z]{2}$`)
	regionPattern     = regexp.MustCompile(`^[A-Z]{2}$`)
	enumPattern       = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)
	decimalPattern    = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
	allowedMediaTypes = map[string]struct{}{
		"text/html": {}, "image/gif": {}, "image/jpeg": {}, "image/png": {},
		"video/mp4": {}, "video/quicktime": {},
	}
	extensionMediaTypes = map[string]string{
		".html": "text/html", ".gif": "image/gif", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".mp4": "video/mp4", ".mov": "video/quicktime",
	}
	webDisallowedCountries = toSet([]string{
		"AD", "AT", "AW", "AX", "BE", "BL", "BQ", "CH", "CW", "DE", "DK", "ES", "FI", "FO", "FR", "GB", "GF", "GG", "GI", "GL", "GP",
		"GR", "IE", "IM", "IS", "IT", "JE", "LI", "LU", "MC", "MF", "MQ", "MT", "NC", "NL", "NO", "PF", "PM", "PT", "RE", "SE", "SH",
		"SJ", "SM", "SX", "VA", "WF", "YT",
	})
)

func validAccountType(value AccountType) bool {
	return value == AccountTypeApp || value == AccountTypeWeb
}
func validNumericID(value string) bool { return numericIDPattern.MatchString(value) }

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validText(value string, maximum int) bool { return validOpaque(value, maximum) }

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func validApproval(accountType string, scopes []string) bool {
	return accountType == "" && (len(scopes) == 0 || len(scopes) == 1 && scopes[0] == approvalScope)
}

func toSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validAbsoluteURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func parseDateTime(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04:05Z"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func validSchedule(start, end string, continuous *bool) bool {
	startTime, ok := parseDateTime(start)
	if !ok {
		return false
	}
	if continuous != nil && *continuous && end != "" || continuous != nil && !*continuous && end == "" {
		return false
	}
	if end == "" {
		return true
	}
	endTime, ok := parseDateTime(end)
	return ok && startTime.Before(endTime)
}

func validUpdateSchedule(end *string, continuous *bool) bool {
	if continuous != nil && *continuous && end != nil || continuous != nil && !*continuous && end == nil {
		return false
	}
	if end == nil {
		return true
	}
	_, ok := parseDateTime(*end)
	return ok
}

func validDecimal(value string, allowZero bool) bool {
	if !decimalPattern.MatchString(value) || len(value) > 64 {
		return false
	}
	number, ok := new(big.Rat).SetString(value)
	return ok && (number.Sign() > 0 || allowZero && number.Sign() == 0)
}

func decimalAtMost(value string, maximum *big.Rat) bool {
	number, ok := new(big.Rat).SetString(value)
	return ok && number.Cmp(maximum) <= 0
}

func validCountryValues(values map[string]string, allowZero bool) bool {
	if len(values) == 0 || len(values) > 256 {
		return false
	}
	for country, value := range values {
		if !countryPattern.MatchString(country) || !validDecimal(value, allowZero) {
			return false
		}
	}
	return true
}

func validBudget(value Budget) bool {
	uniform := value.DailyBudgetForAllCountries != ""
	perCountry := len(value.CountryCodeToDailyBudget) > 0
	return uniform != perCountry && (!uniform || validDecimal(value.DailyBudgetForAllCountries, false)) &&
		(!perCountry || validCountryValues(value.CountryCodeToDailyBudget, false))
}

func validGoalValues(value Goal) bool {
	uniform := value.GoalValueForAllCountries != ""
	perCountry := len(value.CountryCodeToGoalValue) > 0
	return uniform != perCountry && (!uniform || validDecimal(value.GoalValueForAllCountries, true)) &&
		(!perCountry || validCountryValues(value.CountryCodeToGoalValue, true))
}

func goalValues(value Goal) []string {
	if value.GoalValueForAllCountries != "" {
		return []string{value.GoalValueForAllCountries}
	}
	result := make([]string, 0, len(value.CountryCodeToGoalValue))
	for _, item := range value.CountryCodeToGoalValue {
		result = append(result, item)
	}
	return result
}

func isROASGoal(goal GoalType) bool {
	return goal == GoalAdROAS || goal == GoalIAPROAS || goal == GoalMixROAS
}

func validAppGoal(value Goal, bidding BiddingStrategy) bool {
	validType := value.GoalType == GoalCPI || value.GoalType == GoalCPE || value.GoalType == GoalCPP || isROASGoal(value.GoalType)
	if !validType || !validGoalValues(value) || value.GoalType == GoalCPE && value.EventTarget == "" || value.GoalType != GoalCPE && value.EventTarget != "" {
		return false
	}
	if isROASGoal(value.GoalType) != (value.ROASDayTarget != "") || value.ROASDayTarget != "" && value.ROASDayTarget != ROASDay7 && value.ROASDayTarget != ROASDay28 {
		return false
	}
	if bidding == BiddingTargetGoalCPI && value.GoalType == GoalCPE {
		return false
	}
	if isROASGoal(value.GoalType) {
		maximum := big.NewRat(9999, 100)
		for _, item := range goalValues(value) {
			if !decimalAtMost(item, maximum) {
				return false
			}
		}
	}
	return true
}

func validWebGoal(value Goal) bool {
	if value.GoalType != GoalCPE && value.GoalType != GoalCPP && value.GoalType != GoalIAPROAS || !validGoalValues(value) {
		return false
	}
	if value.GoalType == GoalCPE {
		return value.EventTarget != "" && value.ROASDayTarget == ""
	}
	return value.EventTarget == "" && (value.ROASDayTarget == ROASDay0 || value.ROASDayTarget == ROASDay7)
}

func validTargeting(values []Targeting) bool {
	if len(values) == 0 || len(values) > 256 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, target := range values {
		if !countryPattern.MatchString(target.CountryCode) || len(target.RegionCodes) > 0 && len(target.MetroNames) > 0 ||
			(len(target.RegionCodes) > 0 || len(target.MetroNames) > 0) && target.CountryCode != "US" {
			return false
		}
		if _, exists := seen[target.CountryCode]; exists {
			return false
		}
		seen[target.CountryCode] = struct{}{}
		for _, region := range target.RegionCodes {
			if !regionPattern.MatchString(region) {
				return false
			}
		}
		for _, metro := range target.MetroNames {
			if !validText(metro, 256) {
				return false
			}
		}
	}
	return true
}

func validTracking(value Tracking) bool {
	validMethod := value.TrackingMethod == TrackingAdjust || value.TrackingMethod == TrackingAppsFlyer || value.TrackingMethod == TrackingKochava ||
		value.TrackingMethod == TrackingBranch || value.TrackingMethod == TrackingSingular || value.TrackingMethod == TrackingTenjin
	return validMethod && validAbsoluteURL(value.ImpressionURL) && validAbsoluteURL(value.ClickURL)
}

func validList(input ListRequest) bool {
	return input.Page >= 0 && input.Size >= 0 && input.Size <= maximumListSize && len(input.IDs) <= maximumListSize &&
		len(input.HashedIDs) <= maximumListSize && !(len(input.IDs) > 0 && len(input.HashedIDs) > 0) &&
		validStringIDs(input.IDs, true) && validStringIDs(input.HashedIDs, false)
}

func validStringIDs(values []string, numeric bool) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if numeric && !validNumericID(value) || !numeric && (!validOpaque(value, 256) || strings.Contains(value, ",")) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func normalizedPage(page, size int) (int, int) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = maximumListSize
	}
	return page, size
}

func validStatus(value Status) bool { return value == StatusLive || value == StatusPaused }

func validCountries(values []string, accountType AccountType) bool {
	seen := make(map[string]struct{}, len(values))
	for _, country := range values {
		if !countryPattern.MatchString(country) {
			return false
		}
		if accountType == AccountTypeWeb {
			if _, blocked := webDisallowedCountries[country]; blocked {
				return false
			}
		}
		if _, exists := seen[country]; exists {
			return false
		}
		seen[country] = struct{}{}
	}
	return true
}

func validLanguages(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, language := range values {
		if !enumPattern.MatchString(language) {
			return false
		}
		if _, exists := seen[language]; exists {
			return false
		}
		seen[language] = struct{}{}
	}
	return true
}

func validAssetRefs(values []AssetRef, accountType AccountType) bool {
	if len(values) == 0 || len(values) > 50 {
		return false
	}
	seen, counts, declared := make(map[string]struct{}, len(values)), make(map[CreativeAssetType]int), 0
	for _, asset := range values {
		if !validNumericID(asset.ID) {
			return false
		}
		if _, exists := seen[asset.ID]; exists {
			return false
		}
		seen[asset.ID] = struct{}{}
		if asset.Type != "" {
			if !validAssetType(asset.Type, accountType) {
				return false
			}
			counts[asset.Type]++
			declared++
			if counts[asset.Type] > 10 {
				return false
			}
		}
	}
	if declared != len(values) {
		return true
	}
	video := counts[AssetShortVideoPortrait] + counts[AssetLongVideoPortrait]
	if accountType == AccountTypeApp {
		return counts[AssetHostedHTML] > 0 || counts[AssetBanner] > 0 || video > 0 && counts[AssetInterstitialPortrait] > 0
	}
	return video > 0 && (counts[AssetHostedHTML] > 0 || counts[AssetInterstitialPortrait] > 0)
}

func validAssetType(value CreativeAssetType, accountType AccountType) bool {
	if value == AssetHostedHTML || value == AssetInterstitialPortrait || value == AssetShortVideoPortrait || value == AssetLongVideoPortrait {
		return true
	}
	return accountType == AccountTypeApp && value == AssetBanner
}

func validPositiveIDs(values []int64, maximum int) bool {
	if len(values) == 0 || len(values) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func normalizedMediaType(file UploadFile) (string, bool) {
	mediaType := strings.TrimSpace(strings.ToLower(file.ContentType))
	if mediaType != "" {
		parsed, _, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return "", false
		}
		mediaType = parsed
	} else {
		mediaType = extensionMediaTypes[strings.ToLower(filepath.Ext(file.Filename))]
	}
	_, ok := allowedMediaTypes[mediaType]
	return mediaType, ok
}

func validUploadFilename(value string) bool {
	return validText(value, 255) && value != "." && value != ".." && !strings.ContainsAny(value, `/\`)
}

func validateUploadFiles(files []UploadFile) ([]string, error) {
	if len(files) == 0 || len(files) > maximumUploadFiles {
		return nil, invalidArgument("asset_upload", "upload requires between 1 and 40 files")
	}
	mediaTypes := make([]string, len(files))
	seen := make(map[string]struct{}, len(files))
	var total int64
	for index, file := range files {
		if !validUploadFilename(file.Filename) || file.Reader == nil || file.Size <= 0 || file.Size > maximumUploadFileSize {
			return nil, invalidArgument("asset_upload", "file name, reader, or declared size is invalid")
		}
		key := strings.ToLower(file.Filename)
		if _, exists := seen[key]; exists {
			return nil, invalidArgument("asset_upload", "filenames must be unique within a batch")
		}
		seen[key] = struct{}{}
		total += file.Size
		if total > maximumUploadTotal {
			return nil, invalidArgument("asset_upload", "upload batch exceeds 10 GiB")
		}
		mediaType, ok := normalizedMediaType(file)
		if !ok {
			return nil, invalidArgument("asset_upload", "file Content-Type is not supported")
		}
		mediaTypes[index] = mediaType
	}
	return mediaTypes, nil
}

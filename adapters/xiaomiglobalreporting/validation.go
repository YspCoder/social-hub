package xiaomiglobalreporting

import (
	"encoding/json"
	"errors"
	"mime"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var (
	errInvalidReportResult = errors.New("Xiaomi returned an invalid report result")
	errInvalidReportRow    = errors.New("Xiaomi returned an invalid report row")
	errInvalidNameResult   = errors.New("Xiaomi returned an invalid name result")
)

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

func validCookieValueReference(value string) bool {
	return validOpaque(value, 4096)
}

func validCookieValue(value string) bool {
	if len(value) == 0 || len(value) > 16_384 {
		return false
	}
	for index := range value {
		character := value[index]
		if character < 0x21 || character >= 0x7f || character == '"' || character == ',' ||
			character == ';' || character == '\\' {
			return false
		}
	}
	return true
}

func validJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func validIDList(values []int64, maximum int, allowEmpty bool) bool {
	if len(values) == 0 {
		return allowEmpty
	}
	if len(values) > maximum {
		return false
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func authorizedIDs(requested, configured []int64, authorized map[int64]struct{}) ([]int64, bool) {
	if len(requested) == 0 {
		return append([]int64(nil), configured...), true
	}
	if !validIDList(requested, maximumNameIDs, false) {
		return nil, false
	}
	for _, id := range requested {
		if _, found := authorized[id]; !found {
			return nil, false
		}
	}
	return append([]int64(nil), requested...), true
}

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", string(value))
	return err == nil
}

func validDateRange(begin, end Date) bool {
	if !validDate(begin) || !validDate(end) || begin > end {
		return false
	}
	start, _ := time.Parse("2006-01-02", string(begin))
	finish, _ := time.Parse("2006-01-02", string(end))
	return finish.Sub(start) <= 6*24*time.Hour
}

func validDimensions(values []Dimension) bool {
	if len(values) < 2 || len(values) > 7 {
		return false
	}
	seen := make(map[Dimension]struct{}, len(values))
	hasDate, hasBreakdown := false, false
	for _, value := range values {
		switch value {
		case DimensionCampaign, DimensionAdGroup, DimensionAd, DimensionRegion,
			DimensionPlacement, DimensionPublisher:
			hasBreakdown = true
		case DimensionDate:
			hasDate = true
		default:
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return hasDate && hasBreakdown
}

func validStringList(values []string, maximum, itemMaximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, itemMaximum) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validRegions(values []string) bool {
	if len(values) > maximumNameIDs {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) != 2 || value[0] < 'A' || value[0] > 'Z' || value[1] < 'A' || value[1] > 'Z' {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) ([]socialhub.CallOption, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) != 0 {
		return nil, invalidArgument(operation, "only per-call timeouts are supported by Xiaomi Global Reporting API")
	}
	if resolved.Timeout == 0 {
		return nil, nil
	}
	return []socialhub.CallOption{socialhub.WithCallTimeout(resolved.Timeout)}, nil
}

func (client *Client) validateReportQuery(operation string, input ReportQuery) ([]int64, error) {
	accountIDs, ok := authorizedIDs(input.AccountIDs, client.authorizedAccountIDs, client.authorizedAccounts)
	if !ok {
		return nil, invalidArgument(operation, "account_ids must be a unique subset of the configured authorized account IDs")
	}
	if input.Page < 0 || input.Page > 1_000_000 || input.PageSize < 0 || input.PageSize > maximumPageSize {
		return nil, invalidArgument(operation, "page must be nonnegative and page_size must not exceed 1000")
	}
	if input.AdType != AdTypeEffect && input.AdType != AdTypeBrand {
		return nil, invalidArgument(operation, "ad_type must be Effect or Brand")
	}
	if input.Language != LanguageSimplifiedChinese && input.Language != LanguageEnglish {
		return nil, invalidArgument(operation, "language must be zh_CN or en_US")
	}
	if !validDateRange(input.Begin, input.End) {
		return nil, invalidArgument(operation, "begin and end must be UTC dates spanning no more than seven days")
	}
	if !validDimensions(input.Dimensions) {
		return nil, invalidArgument(operation, "dimensions must contain Date, at least one reporting dimension, and no duplicates")
	}
	for _, values := range [][]int64{input.CampaignIDs, input.AdGroupIDs, input.CreativeIDs} {
		if !validIDList(values, maximumNameIDs, true) {
			return nil, invalidArgument(operation, "campaign, ad group, or creative IDs are invalid")
		}
	}
	if !validStringList(input.PlacementIDs, maximumNameIDs, 256) ||
		!validStringList(input.PublisherIDs, maximumNameIDs, 256) || !validRegions(input.Regions) {
		return nil, invalidArgument(operation, "placement, publisher, or region filters are invalid")
	}
	return accountIDs, nil
}

func (client *Client) validateNameQuery(operation string, input NameQuery) ([]int64, error) {
	accountIDs, ok := authorizedIDs(input.AccountIDs, client.authorizedAccountIDs, client.authorizedAccounts)
	if !ok {
		return nil, invalidArgument(operation, "account_ids must be a unique subset of the configured authorized account IDs")
	}
	for _, values := range [][]int64{input.CampaignIDs, input.AdGroupIDs, input.CreativeIDs} {
		if !validIDList(values, maximumNameIDs, true) {
			return nil, invalidArgument(operation, "each name lookup level accepts at most 8000 unique positive IDs")
		}
	}
	return accountIDs, nil
}

func validReportRow(row ReportRow, authorized map[int64]struct{}) bool {
	if len(row) == 0 || len(row) > 512 {
		return false
	}
	for key, value := range row {
		if !validFieldName(key) || len(value.raw) == 0 || len(value.raw) > maximumReportValueBytes || !json.Valid(value.raw) {
			return false
		}
	}
	accountValue, found := row["accountId"]
	if !found {
		return false
	}
	accountID, err := reportInt64(accountValue)
	if err != nil {
		return false
	}
	_, found = authorized[accountID]
	return found
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && character != '-' && character != '.' &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func reportInt64(value ReportValue) (int64, error) {
	trimmed := strings.TrimSpace(string(value.raw))
	if strings.HasPrefix(trimmed, "\"") {
		var text string
		if err := json.Unmarshal(value.raw, &text); err != nil {
			return 0, err
		}
		trimmed = text
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errInvalidReportRow
	}
	return parsed, nil
}

func validNameDirectory(accounts []AccountNames, authorized map[int64]struct{}) bool {
	if len(accounts) > maximumNameIDs {
		return false
	}
	seenAccounts := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		if _, found := authorized[account.AccountID]; !found || !validPlatformText(account.AccountName, 4096) {
			return false
		}
		if _, found := seenAccounts[account.AccountID]; found {
			return false
		}
		seenAccounts[account.AccountID] = struct{}{}
		if len(account.Campaigns) > maximumNameIDs || len(account.AdGroups) > maximumNameIDs || len(account.Creatives) > maximumNameIDs {
			return false
		}
		if !validCampaignNames(account.Campaigns) || !validAdGroupNames(account.AdGroups) || !validCreativeNames(account.Creatives) {
			return false
		}
	}
	return true
}

func validCampaignNames(values []CampaignName) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value.ID <= 0 || !validPlatformText(value.Name, 4096) {
			return false
		}
		if _, found := seen[value.ID]; found {
			return false
		}
		seen[value.ID] = struct{}{}
	}
	return true
}

func validAdGroupNames(values []AdGroupName) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value.ID <= 0 || !validPlatformText(value.Name, 4096) {
			return false
		}
		if _, found := seen[value.ID]; found {
			return false
		}
		seen[value.ID] = struct{}{}
	}
	return true
}

func validCreativeNames(values []CreativeName) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value.ID <= 0 || !validPlatformText(value.Name, 4096) {
			return false
		}
		if _, found := seen[value.ID]; found {
			return false
		}
		seen[value.ID] = struct{}{}
	}
	return true
}

func validPlatformText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

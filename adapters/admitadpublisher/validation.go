package admitadpublisher

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validBaseURL(value string) bool {
	return validEndpoint(value) && !strings.HasSuffix(value, "/")
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validOAuthScopes(scopes []string) bool {
	if len(scopes) > 64 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if !validOpaque(scope, 256) || strings.ContainsAny(scope, " \t\r\n") {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func containsScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func validPagination(offset, limit int) bool {
	return offset >= 0 && limit >= 0 && limit <= 500
}

func effectivePageLimit(limit int) int {
	if limit == 0 {
		return 20
	}
	return limit
}

func positiveExactID(value ExactValue) (int64, bool) {
	parsed, err := strconv.ParseInt(value.String(), 10, 64)
	return parsed, err == nil && parsed > 0
}

func validConnectionStatus(value ConnectionStatus) bool {
	return value == "" || value == ConnectionActive || value == ConnectionPending || value == ConnectionDeclined
}

func validProgramTool(value ProgramTool) bool {
	switch value {
	case "", ProgramToolDeeplink, ProgramToolProducts, ProgramToolRetag, ProgramToolLostOrders,
		ProgramToolCoupons, ProgramToolBasketTracking, ProgramToolMobileSiteTracking, ProgramToolMobileAppTracking:
		return true
	default:
		return false
	}
}

func validListPrograms(input ListProgramsRequest) bool {
	return input.WebsiteID > 0 && validConnectionStatus(input.ConnectionStatus) &&
		validProgramTool(input.HasTool) && validPagination(input.Offset, input.Limit)
}

func validGetProgram(input GetProgramRequest) bool {
	return input.WebsiteID > 0 && input.CampaignID > 0
}

func validTargetURL(value string) bool {
	if !validOpaque(value, 8192) || strings.Contains(strings.ToLower(value), "%00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validSubID(value string, maximumRunes int) bool {
	return value == "" || (validOpaque(value, 4*maximumRunes) && utf8.RuneCountInString(value) <= maximumRunes &&
		!strings.Contains(strings.ToLower(value), "%00"))
}

func validGenerateDeeplinks(input GenerateDeeplinksRequest) bool {
	if input.WebsiteID <= 0 || input.CampaignID <= 0 || len(input.TargetURLs) == 0 || len(input.TargetURLs) > 200 ||
		!validSubID(input.SubID, 50) || !validSubID(input.SubID1, 50) || !validSubID(input.SubID2, 50) ||
		!validSubID(input.SubID3, 50) || !validSubID(input.SubID4, 120) {
		return false
	}
	for _, target := range input.TargetURLs {
		if !validTargetURL(target) {
			return false
		}
	}
	return true
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 35 || value[0] == '-' || value[len(value)-1] == '-' || strings.Contains(value, "--") {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '-' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validOrderToken(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	if value[0] == '-' {
		value = value[1:]
	}
	if value == "" {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validOrderTokens(values []string) bool {
	if len(values) > 16 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validCouponOrder(value) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCouponOrder(value string) bool {
	if !validOrderToken(value) {
		return false
	}
	value = strings.TrimPrefix(value, "-")
	return value == "name" || value == "rating" || value == "date_start" || value == "date_end"
}

func validListCoupons(input ListCouponsRequest) bool {
	if input.WebsiteID <= 0 || input.CampaignID < 0 || input.CategoryID < 0 || input.CampaignCategoryID < 0 ||
		input.TypeID < 0 || !validOptionalOpaque(input.Region, 64) || !validOptionalOpaque(input.Search, 512) || !validLanguage(input.Language) ||
		!validOrderTokens(input.OrderBy) || !validPagination(input.Offset, input.Limit) ||
		(input.CustomerType != "" && input.CustomerType != CustomerNew && input.CustomerType != CustomerAll) {
		return false
	}
	return input.DateStart.IsZero() || input.DateEnd.IsZero() || !input.DateEnd.Before(input.DateStart)
}

func validPositiveIDs(values []int64) bool {
	if len(values) > 100 {
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

func validStatisticsOrder(value StatisticsOrder) bool {
	text := string(value)
	if strings.HasPrefix(text, "-") {
		text = text[1:]
	}
	switch StatisticsOrder(text) {
	case StatisticsOrderAction, StatisticsOrderName, StatisticsOrderLeads, StatisticsOrderSales, StatisticsOrderPayment,
		StatisticsOrderPaymentApproved, StatisticsOrderPaymentDeclined, StatisticsOrderPaymentOpen,
		StatisticsOrderViews, StatisticsOrderClicks, StatisticsOrderCTR, StatisticsOrderECPC,
		StatisticsOrderCR, StatisticsOrderECPM:
		return true
	default:
		return false
	}
}

func validListCampaignStatistics(input ListCampaignStatisticsRequest) bool {
	if !validPagination(input.Offset, input.Limit) || !validPositiveIDs(input.WebsiteIDs) ||
		!validPositiveIDs(input.CampaignIDs) || !validOptionalOpaque(input.SubID, 120) || len(input.OrderBy) > 16 {
		return false
	}
	if !input.DateStart.IsZero() && !input.DateEnd.IsZero() && input.DateEnd.Before(input.DateStart) {
		return false
	}
	seen := make(map[StatisticsOrder]struct{}, len(input.OrderBy))
	for _, order := range input.OrderBy {
		if !validStatisticsOrder(order) {
			return false
		}
		if _, exists := seen[order]; exists {
			return false
		}
		seen[order] = struct{}{}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "read-only Publisher API workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Publisher API endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

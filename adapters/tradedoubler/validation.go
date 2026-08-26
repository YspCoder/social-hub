package tradedoubler

import (
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maximumMatrixPathBytes = 8 << 10

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
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

func validPublisherToken(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				if character < 'A' || character > 'F' {
					return false
				}
			}
		}
	}
	return true
}

func positiveExactID(value ExactValue) (int64, bool) {
	parsed, err := strconv.ParseInt(value.String(), 10, 64)
	return parsed, err == nil && parsed > 0
}

func nonNegativeExactInteger(value ExactValue) bool {
	parsed, err := strconv.ParseInt(value.String(), 10, 64)
	return err == nil && parsed >= 0
}

func validRequiredWebURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && validOpaque(value, 16_384)
}

func validPositiveIDs(values []int64, required bool) bool {
	if required && len(values) == 0 {
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

func validStrings(values []string, maximum int) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximum) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			return false
		}
	}
	return true
}

func parseOptionalPrice(value string) (float64, bool) {
	if value == "" {
		return 0, true
	}
	if !validOpaque(value, 64) {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil && parsed >= 0 && !math.IsInf(parsed, 0) && !math.IsNaN(parsed)
}

func validOrder(value ProductOrder) bool {
	switch value {
	case "", OrderPriceAscending, OrderPriceDescending, OrderModificationAscending, OrderModificationDescending:
		return true
	default:
		return false
	}
}

func validPagination(page, pageSize, limit int) bool {
	if page < 0 || pageSize < 0 || limit < 0 || pageSize > MaximumSearchProducts || limit > MaximumSearchProducts {
		return false
	}
	if pageSize == 0 {
		return page == 0
	}
	if limit == 0 || page > MaximumSearchProducts/pageSize-1 {
		return false
	}
	return limit >= (page+1)*pageSize
}

func validSearchProducts(input SearchProductsRequest) bool {
	minimumPrice, minimumOK := parseOptionalPrice(input.MinPrice)
	maximumPrice, maximumOK := parseOptionalPrice(input.MaxPrice)
	if !minimumOK || !maximumOK || input.MinPrice != "" && input.MaxPrice != "" && maximumPrice < minimumPrice {
		return false
	}
	if !validPositiveIDs(input.FeedIDs, true) || !validPositiveIDs(input.TDCategoryIDs, false) ||
		!validOptionalOpaque(input.Keyword, 2048) || !validCurrency(input.Currency) ||
		!validOptionalOpaque(input.MinUpdateDate, 128) || !validOptionalOpaque(input.MaxUpdateDate, 128) ||
		!validStrings(input.Brands, 1024) || !validLanguage(input.Language) || !validOrder(input.OrderBy) ||
		!validPagination(input.Page, input.PageSize, input.Limit) {
		return false
	}
	return input.DateOutputFormat == "" || input.DateOutputFormat == DateOutputISO8601
}

func validateProductsResponse(operation string, response ProductsResponse, input SearchProductsRequest) error {
	if response.Products == nil || len(response.Products) > MaximumSearchProducts ||
		!nonNegativeExactInteger(response.Header.TotalHits) {
		return platformContractError(operation, "Tradedoubler omitted the product collection, exceeded the search cap, or returned an invalid total-hit count")
	}
	requestedFeeds := make(map[int64]struct{}, len(input.FeedIDs))
	for _, feedID := range input.FeedIDs {
		requestedFeeds[feedID] = struct{}{}
	}
	offerIDs := make(map[string]struct{})
	for _, product := range response.Products {
		if product.Offers == nil || product.Categories == nil || len(product.Offers) == 0 {
			return platformContractError(operation, "Tradedoubler returned a product without required offer or category collections")
		}
		for _, offer := range product.Offers {
			feedID, validFeedID := positiveExactID(offer.FeedID)
			if !validFeedID || !validOpaque(offer.ID, maxExactValueBytes) ||
				!validOpaque(offer.SourceProductID, 4096) || !validRequiredWebURL(offer.ProductURL) {
				return platformContractError(operation, "Tradedoubler returned a product offer with invalid identity fields")
			}
			if _, requested := requestedFeeds[feedID]; !requested {
				return platformContractError(operation, "Tradedoubler returned a product offer from an unrequested feed")
			}
			if _, duplicate := offerIDs[offer.ID]; duplicate {
				return platformContractError(operation, "Tradedoubler returned a duplicate product offer ID")
			}
			offerIDs[offer.ID] = struct{}{}
		}
	}
	return nil
}

func validateProductFeedsResponse(operation string, response ProductFeedsResponse, input ListProductFeedsRequest) error {
	if response.Feeds == nil {
		return platformContractError(operation, "Tradedoubler omitted the product-feed collection")
	}
	requestedPrograms := make(map[int64]struct{}, len(input.ProgramIDs))
	for _, programID := range input.ProgramIDs {
		requestedPrograms[programID] = struct{}{}
	}
	feedIDs := make(map[int64]struct{}, len(response.Feeds))
	for _, feed := range response.Feeds {
		feedID, valid := positiveExactID(feed.FeedID)
		if !valid {
			return platformContractError(operation, "Tradedoubler returned a product feed without a valid ID")
		}
		if _, duplicate := feedIDs[feedID]; duplicate {
			return platformContractError(operation, "Tradedoubler returned a duplicate product-feed ID")
		}
		feedIDs[feedID] = struct{}{}
		if feed.Programs == nil {
			return platformContractError(operation, "Tradedoubler omitted a product feed's program collection")
		}
		programIDs := make(map[int64]struct{}, len(feed.Programs))
		matchedRequestedProgram := len(requestedPrograms) == 0
		for _, program := range feed.Programs {
			programID, valid := positiveExactID(program.ProgramID)
			if !valid {
				return platformContractError(operation, "Tradedoubler returned a product feed with an invalid program ID")
			}
			if _, duplicate := programIDs[programID]; duplicate {
				return platformContractError(operation, "Tradedoubler returned duplicate program summaries for a product feed")
			}
			programIDs[programID] = struct{}{}
			if _, requested := requestedPrograms[programID]; requested {
				matchedRequestedProgram = true
			}
		}
		if !matchedRequestedProgram {
			return platformContractError(operation, "Tradedoubler returned a product feed outside the requested program filter")
		}
	}
	return nil
}

func validateUnlimitedFeedLastUpdatedResponse(
	operation string,
	response UnlimitedFeedLastUpdatedResponse,
	requestedFeedID int64,
) error {
	if response.FeedIDs == nil || len(response.FeedIDs) != 1 {
		return platformContractError(operation, "Tradedoubler did not return exactly one unlimited-feed ID")
	}
	feedID, valid := positiveExactID(response.FeedIDs[0])
	if !valid || feedID != requestedFeedID {
		return platformContractError(operation, "Tradedoubler returned a mismatched unlimited-feed ID")
	}
	if !validLastUpdatedTime(response.LastUpdatedTime) {
		return platformContractError(operation, "Tradedoubler returned an invalid unlimited-feed last-updated timestamp")
	}
	return nil
}

func validLastUpdatedTime(value string) bool {
	layout := ""
	switch len(value) {
	case len("2006-01-02T15:04"):
		layout = "2006-01-02T15:04"
	case len("2006-01-02T15:04:05"):
		layout = "2006-01-02T15:04:05"
	case len("2006-01-02T15:04:05.000"):
		layout = "2006-01-02T15:04:05.000"
	case len("2006-01-02T15:04:05.000000"):
		layout = "2006-01-02T15:04:05.000000"
	case len("2006-01-02T15:04:05.000000000"):
		layout = "2006-01-02T15:04:05.000000000"
	default:
		return false
	}
	_, err := time.Parse(layout, value)
	return err == nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Products API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "bodyless Products API read workflows do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "Products API endpoints do not define response field selection")
	}
	return nil
}

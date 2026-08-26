package rakutenadvertising

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumDateWindow      = 30 * 24 * time.Hour
	maximumProviderIDBytes = 4096
)

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

func validOptionalText(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validPositiveID(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed > 0 && len(value) <= 20
}

func validHTTPURL(value string) bool {
	if !validOpaque(value, 8192) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validSearchAdvertisers(input SearchAdvertisersRequest) bool {
	return input.Page >= 0 && input.Limit >= 0 && input.Limit <= 200 && input.Network >= 0 &&
		validOptionalText(input.ShipsTo, 256)
}

func validPartnerStatus(value PartnerStatus) bool {
	switch value {
	case "", PartnerActive, PartnerPending, PartnerSelfRemoved, PartnerPermanentDecline,
		PartnerPermanentRemove, PartnerTemporaryDecline, PartnerTemporaryRemove, PartnerExtended:
		return true
	default:
		return false
	}
}

func validAdvertiserStatus(value AdvertiserStatus) bool {
	return value == "" || value == AdvertiserActive || value == AdvertiserInactive
}

func validDateRange(value DateRange) bool {
	return value == "" || value == DateRangeOneDay || value == DateRangeSevenDays || value == DateRangeThirtyDays
}

func validPartnershipSort(value PartnershipSortField) bool {
	return value == "" || value == SortByApplyDate || value == SortByApproveDate || value == SortByStatusUpdateDate
}

func validSortDirection(value SortDirection) bool {
	return value == "" || value == SortAscending || value == SortDescending
}

func validPartnershipNetwork(value int) bool {
	if value == 0 {
		return true
	}
	for _, candidate := range []int{1, 3, 5, 7, 8, 9, 41} {
		if value == candidate {
			return true
		}
	}
	return false
}

func validListPartnerships(input ListPartnershipsRequest) bool {
	return validPartnerStatus(input.PartnerStatus) && validPartnershipNetwork(input.Network) &&
		validAdvertiserStatus(input.AdvertiserStatus) && validOptionalText(input.Category, 1024) &&
		validDateRange(input.StatusUpdateRange) && validDateRange(input.ApproveDateRange) &&
		validDateRange(input.ApplyDateRange) && validPartnershipSort(input.SortBy) && validSortDirection(input.OrderBy) &&
		input.Limit >= 0 && input.Limit <= 200 && input.Page >= 0
}

func validProductLanguage(value ProductLanguage) bool {
	return value == "" || value == LanguageEnglishUS || value == LanguageFrenchFrance ||
		value == LanguageGermanGermany || value == LanguagePortugueseBR
}

func validProductSortField(value ProductSortField) bool {
	return value == ProductSortRetailPrice || value == ProductSortName || value == ProductSortCategory || value == ProductSortAdvertiser
}

func validProductSearchTerm(value string) bool {
	return validOptionalText(value, 4096) && !strings.ContainsAny(value, "&=?{}\\()[]-;~|$!><*%")
}

func validSearchProducts(input SearchProductsRequest) bool {
	if !validProductSearchTerm(input.Keyword) || !validProductSearchTerm(input.Exact) ||
		!validProductSearchTerm(input.One) || !validProductSearchTerm(input.None) ||
		!validOptionalText(input.Category, 4096) || !validProductLanguage(input.Language) ||
		input.Max < 0 || input.Max > 100 || input.PageNumber < 0 || input.AdvertiserID < 0 || len(input.Sort) > 4 {
		return false
	}
	seen := make(map[ProductSortField]struct{}, len(input.Sort))
	for _, sort := range input.Sort {
		if !validProductSortField(sort.Field) || !validSortDirection(sort.Direction) || sort.Direction == "" {
			return false
		}
		if _, found := seen[sort.Field]; found {
			return false
		}
		seen[sort.Field] = struct{}{}
	}
	return true
}

func validCreateDeepLink(input CreateDeepLinkRequest) bool {
	return input.AdvertiserID > 0 && validHTTPURL(input.URL) && validOptionalText(input.U1, 4096)
}

func validCurrency(value Currency) bool {
	switch value {
	case "", CurrencyUSD, CurrencyCAD, CurrencyGBP, CurrencyEUR, CurrencyBRL, CurrencyAUD, CurrencyJPY:
		return true
	default:
		return false
	}
}

func validTransactionType(value TransactionType) bool {
	return value == "" || value == TransactionBatch || value == TransactionRealtime
}

func validDatePair(start, end time.Time, oldest time.Time) bool {
	if start.IsZero() != end.IsZero() {
		return false
	}
	if start.IsZero() {
		return true
	}
	return end.After(start) && end.Sub(start) <= maximumDateWindow && !start.Before(oldest)
}

func validListTransactions(input ListTransactionsRequest, now time.Time) bool {
	now = now.UTC().Truncate(time.Second)
	return validDatePair(input.ProcessDateStart, input.ProcessDateEnd, now.Add(-30*24*time.Hour)) &&
		validDatePair(input.TransactionDateStart, input.TransactionDateEnd, now.Add(-100*24*time.Hour)) &&
		input.Limit >= 0 && input.Page >= 0 && validCurrency(input.Currency) && validTransactionType(input.Type)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Affiliate APIs endpoints do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Affiliate APIs endpoints do not define field selection")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func validateAdvertisersResponse(operation string, response AdvertisersResponse) error {
	if response.Advertisers == nil {
		return platformContractError(operation, "Rakuten Advertising omitted the advertisers collection")
	}
	seen := make(map[int64]struct{}, len(response.Advertisers))
	for _, advertiser := range response.Advertisers {
		if advertiser.ID <= 0 {
			return platformContractError(operation, "Rakuten Advertising returned an advertiser without a valid ID")
		}
		if _, found := seen[advertiser.ID]; found {
			return platformContractError(operation, "Rakuten Advertising returned duplicate advertiser IDs")
		}
		seen[advertiser.ID] = struct{}{}
	}
	return nil
}

func validatePartnershipsResponse(operation string, response PartnershipsResponse) error {
	if response.Partnerships == nil {
		return platformContractError(operation, "Rakuten Advertising omitted the partnerships collection")
	}
	seen := make(map[int64]struct{}, len(response.Partnerships))
	for _, partnership := range response.Partnerships {
		identifier := partnership.Advertiser.ID
		if identifier <= 0 {
			return platformContractError(operation, "Rakuten Advertising returned a partnership without a valid advertiser ID")
		}
		if _, found := seen[identifier]; found {
			return platformContractError(operation, "Rakuten Advertising returned duplicate advertiser partnerships")
		}
		seen[identifier] = struct{}{}
	}
	return nil
}

func validateProductSearchResponse(operation string, response ProductSearchResponse) error {
	if response.TotalMatches < 0 || response.TotalPages < 0 || response.PageNumber < 0 {
		return platformContractError(operation, "Rakuten Advertising returned invalid Product Search pagination")
	}
	seen := make(map[string]struct{}, len(response.Products))
	for _, product := range response.Products {
		if product.AdvertiserID <= 0 || !validOpaque(product.LinkID, maximumProviderIDBytes) {
			return platformContractError(operation, "Rakuten Advertising returned a product without valid advertiser and link IDs")
		}
		identifier := strconv.FormatInt(product.AdvertiserID, 10) + ":" + product.LinkID
		if _, found := seen[identifier]; found {
			return platformContractError(operation, "Rakuten Advertising returned duplicate product link IDs")
		}
		seen[identifier] = struct{}{}
	}
	return nil
}

func validateDeepLinkResponse(operation string, response DeepLinkResponse, expectedAdvertiserID int64) error {
	if response.Advertiser.ID != expectedAdvertiserID {
		return platformContractError(operation, "Rakuten Advertising returned a deep link for a different advertiser")
	}
	if !validHTTPURL(response.Advertiser.DeepLink.URL) {
		return platformContractError(operation, "Rakuten Advertising returned an invalid deep-link URL")
	}
	return nil
}

func validateTransactionsResponse(operation string, response TransactionsResponse, expectedPublisherID string) error {
	if response.Transactions == nil {
		return platformContractError(operation, "Rakuten Advertising omitted the transactions collection")
	}
	seen := make(map[string]struct{}, len(response.Transactions))
	for _, transaction := range response.Transactions {
		if !validOpaque(transaction.ETransactionID, maximumProviderIDBytes) {
			return platformContractError(operation, "Rakuten Advertising returned a transaction without a valid ID")
		}
		if transaction.PublisherID.String() != expectedPublisherID {
			return platformContractError(operation, "Rakuten Advertising returned a transaction for a different publisher")
		}
		if _, found := seen[transaction.ETransactionID]; found {
			return platformContractError(operation, "Rakuten Advertising returned duplicate transaction IDs")
		}
		seen[transaction.ETransactionID] = struct{}{}
	}
	return nil
}

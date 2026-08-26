package skimlinks

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validAccountSettings(settings AccountSettings) bool {
	return settings.PublisherID > 0 && settings.PublisherDomainID > 0 && validOpaque(settings.SiteID, 256)
}

func positiveExactID(value ExactValue) (int64, bool) {
	parsed, err := strconv.ParseInt(value.String(), 10, 64)
	return parsed, err == nil && parsed > 0
}

func validExactIdentifier(value ExactValue) bool {
	raw := bytes.TrimSpace(value.Bytes())
	if len(raw) == 0 || value.IsNull() {
		return false
	}
	if raw[0] == '"' {
		return validOpaque(value.String(), maxExactValueBytes)
	}
	parsed, err := strconv.ParseInt(string(raw), 10, 64)
	return err == nil && parsed >= 0
}

func validWebURL(value string, required bool) bool {
	if value == "" {
		return !required
	}
	if !validOpaque(value, 8192) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validCountryCode(value string) bool {
	return value == "" || len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validAnyCaseCountryCode(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' && character < 'a' || character > 'z' {
			return false
		}
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

func validDate(value Date) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	parsed, err := time.Parse("2006-01-02", string(value))
	return err == nil && Date(parsed.Format("2006-01-02")) == value
}

func validTimezone(value string) bool {
	if value == "" || value == "UTC" {
		return true
	}
	if len(value) == 6 && (value[0] == '+' || value[0] == '-') && value[3] == ':' &&
		value[1] >= '0' && value[1] <= '9' && value[2] >= '0' && value[2] <= '9' &&
		value[4] >= '0' && value[4] <= '9' && value[5] >= '0' && value[5] <= '9' {
		hours := int(value[1]-'0')*10 + int(value[2]-'0')
		minutes := int(value[4]-'0')*10 + int(value[5]-'0')
		return hours < 14 && minutes <= 59 || hours == 14 && minutes == 0
	}
	if !validOpaque(value, 255) {
		return false
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func validListMerchants(input ListMerchantsRequest) bool {
	if input.PublisherDomainID < 0 || input.AdvertiserID < 0 || input.MerchantID < 0 || input.VerticalID < 0 ||
		input.AlternativeVerticalID < 0 || input.Limit < 0 || input.Offset < 0 ||
		!validOptionalOpaque(input.Search, 1024) || !validCountryCode(input.Country) ||
		!validOptionalOpaque(input.AlternativeVerticalTaxonomy, 255) || !validCountryCode(input.AlternativeVerticalCountry) {
		return false
	}
	switch input.SortBy {
	case "", MerchantSortName, MerchantSortPartnerType, MerchantSortCalculatedCommissionRate,
		MerchantSortCalculatedECPC, MerchantSortPopularity:
	default:
		return false
	}
	switch input.SortDirection {
	case "", SortAscending, SortDescending:
	default:
		return false
	}
	if input.AlternativeVerticalTaxonomy == "top_merchant" &&
		(input.AlternativeVerticalID == 0 || input.AlternativeVerticalCountry == "") {
		return false
	}
	return true
}

func validWrapLink(input WrapLinkRequest) bool {
	return validWebURL(input.DestinationURL, true) && validWebURL(input.SourceURL, false) &&
		validOptionalOpaque(input.CustomID, 2048)
}

func validListCommissions(input ListCommissionsRequest) bool {
	if input.Limit < 0 || input.Limit > 5000 || input.Offset < 0 || input.MerchantID < 0 ||
		input.AdvertiserID < 0 || input.DomainID < 0 || !validOptionalOpaque(input.CustomID, 2048) ||
		!validOptionalOpaque(input.CommissionID, 1024) {
		return false
	}
	hasStart, hasEnd := !input.StartDate.IsZero(), !input.EndDate.IsZero()
	if hasStart != hasEnd || hasStart && input.EndDate.Before(input.StartDate) {
		return false
	}
	if !hasStart && input.UpdatedSince.IsZero() {
		return false
	}
	switch input.SortDirection {
	case "", ReportSortAscending, ReportSortDescending:
	default:
		return false
	}
	switch input.SortBy {
	case "", CommissionSortID, CommissionSortTransactionDate:
	default:
		return false
	}
	switch input.Status {
	case "", CommissionActive, CommissionCancelled:
	default:
		return false
	}
	switch input.CommissionType {
	case "", CommissionCPA, CommissionCPC, CommissionCPL, CommissionFlatFee, CommissionPerformance:
	default:
		return false
	}
	return true
}

func validPerformanceReport(input PerformanceReportRequest) bool {
	if !validDate(input.StartDate) || !validDate(input.EndDate) || input.EndDate < input.StartDate ||
		input.Limit < 0 || input.Limit > 600 || input.Offset < 0 || input.AdvertiserID < 0 || input.DomainID < 0 ||
		!validCurrency(input.Currency) || !validTimezone(input.Timezone) ||
		!validOptionalOpaque(input.PageSearch, 2048) || !validOptionalOpaque(input.LinkSearch, 2048) ||
		!validOptionalOpaque(input.MerchantSearch, 1024) {
		return false
	}
	switch input.ReportBy {
	case ReportByPage, ReportByDate, ReportByDevice, ReportByCountry, ReportByDomain,
		ReportByLink, ReportByMerchant, ReportByNetworkPayoutType:
	default:
		return false
	}
	switch input.SortDirection {
	case "", ReportSortAscending, ReportSortDescending:
	default:
		return false
	}
	if !validReportSortField(input.SortBy) {
		return false
	}
	switch input.TimePeriod {
	case "":
	case TimePeriodDay, TimePeriodWeek, TimePeriodMonth:
		if input.ReportBy != ReportByDate {
			return false
		}
	default:
		return false
	}
	switch input.PaymentType {
	case "", PaymentTypeAffiliate, PaymentTypeFlatFee:
	default:
		return false
	}
	for _, country := range input.UserCountries {
		if !validAnyCaseCountryCode(country) {
			return false
		}
	}
	return true
}

func validReportSortField(value ReportSortField) bool {
	switch value {
	case "", ReportSortImpressions, ReportSortAffiliatedClicks, ReportSortOrderAmount,
		ReportSortPublisherCommission, ReportSortSales, ReportSortPageURL, ReportSortISODate,
		ReportSortDeviceType, ReportSortUserCountry, ReportSortDomain, ReportSortMerchantName,
		ReportSortTargetURL:
		return true
	default:
		return false
	}
}

func validateMerchantsResponse(
	operation string,
	response MerchantsResponse,
	input ListMerchantsRequest,
	publisherDomainID int64,
) error {
	if response.Merchants == nil || response.NumberReturned < 0 ||
		response.NumberReturned != int64(len(response.Merchants)) {
		return platformContractError(operation, "Skimlinks returned invalid merchant collection metadata")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 200
	}
	if len(response.Merchants) > limit || response.HasMore && !validExactIdentifier(response.NextValue) {
		return platformContractError(operation, "Skimlinks returned invalid merchant pagination")
	}
	advertiserIDs := make(map[int64]struct{}, len(response.Merchants))
	for _, merchant := range response.Merchants {
		advertiserID, valid := positiveExactID(merchant.AdvertiserID)
		if !valid || input.AdvertiserID > 0 && advertiserID != input.AdvertiserID {
			return platformContractError(operation, "Skimlinks returned a merchant with an invalid or mismatched advertiser ID")
		}
		if _, duplicate := advertiserIDs[advertiserID]; duplicate {
			return platformContractError(operation, "Skimlinks returned a duplicate advertiser ID")
		}
		advertiserIDs[advertiserID] = struct{}{}
		if merchant.ID.IsSet() && !merchant.ID.IsNull() {
			identifier, valid := positiveExactID(merchant.ID)
			if !valid || identifier != advertiserID {
				return platformContractError(operation, "Skimlinks returned inconsistent merchant identity fields")
			}
		}
		matchedMerchantID := input.MerchantID == 0
		if merchant.MerchantID.IsSet() && !merchant.MerchantID.IsNull() {
			merchantID, valid := positiveExactID(merchant.MerchantID)
			if !valid {
				return platformContractError(operation, "Skimlinks returned an invalid deprecated merchant ID")
			}
			matchedMerchantID = matchedMerchantID || merchantID == input.MerchantID
		}
		merchantIDs := make(map[int64]struct{}, len(merchant.MerchantIDs))
		for _, value := range merchant.MerchantIDs {
			merchantID, valid := positiveExactID(value)
			if !valid {
				return platformContractError(operation, "Skimlinks returned an invalid merchant program ID")
			}
			if _, duplicate := merchantIDs[merchantID]; duplicate {
				return platformContractError(operation, "Skimlinks returned a duplicate merchant program ID")
			}
			merchantIDs[merchantID] = struct{}{}
			matchedMerchantID = matchedMerchantID || merchantID == input.MerchantID
		}
		if !matchedMerchantID {
			return platformContractError(operation, "Skimlinks returned a merchant outside the requested merchant filter")
		}
	}
	statIDs := make(map[int64]struct{}, len(response.PublisherDomainStats))
	for _, stat := range response.PublisherDomainStats {
		statID, valid := positiveExactID(stat.PublisherDomainID)
		if !valid || statID != publisherDomainID {
			return platformContractError(operation, "Skimlinks returned publisher-domain stats for an unexpected site")
		}
		if _, duplicate := statIDs[statID]; duplicate {
			return platformContractError(operation, "Skimlinks returned duplicate publisher-domain stats")
		}
		statIDs[statID] = struct{}{}
	}
	return nil
}

func validateDomainsResponse(operation string, response DomainsResponse) error {
	if response.Domains == nil || response.NumberReturned < int64(len(response.Domains)) {
		return platformContractError(operation, "Skimlinks returned invalid domain collection metadata")
	}
	if response.HasMore && !validExactIdentifier(response.NextValue) && !validExactIdentifier(response.NextPrefix) {
		return platformContractError(operation, "Skimlinks returned invalid domain pagination")
	}
	domainIDs := make(map[int64]struct{}, len(response.Domains))
	for _, domain := range response.Domains {
		domainID, validDomainID := positiveExactID(domain.ID)
		_, validMerchantID := positiveExactID(domain.MerchantID)
		_, validAdvertiserID := positiveExactID(domain.AdvertiserID)
		if !validDomainID || !validMerchantID || !validAdvertiserID || !validOpaque(domain.Domain, 1024) ||
			strings.Contains(domain.Domain, "://") || strings.ContainsAny(domain.Domain, "/?#") {
			return platformContractError(operation, "Skimlinks returned a merchant domain with invalid identity fields")
		}
		if _, duplicate := domainIDs[domainID]; duplicate {
			return platformContractError(operation, "Skimlinks returned a duplicate merchant-domain ID")
		}
		domainIDs[domainID] = struct{}{}
	}
	return nil
}

func validateCommissionsResponse(
	operation string,
	response CommissionsResponse,
	input ListCommissionsRequest,
	publisherID int64,
) error {
	expectedLimit := input.Limit
	if expectedLimit == 0 {
		expectedLimit = 30
	}
	pagination := response.Pagination
	if response.Commissions == nil || pagination.TotalCount < int64(len(response.Commissions)) ||
		pagination.Offset < 0 || pagination.Limit <= 0 || pagination.Limit > 5000 ||
		int64(len(response.Commissions)) > pagination.Limit || len(response.Commissions) > expectedLimit ||
		pagination.HasNext && len(response.Commissions) == 0 {
		return platformContractError(operation, "Skimlinks returned invalid commission pagination")
	}
	commissionIDs := make(map[string]struct{}, len(response.Commissions))
	for _, commission := range response.Commissions {
		if !validExactIdentifier(commission.CommissionID) {
			return platformContractError(operation, "Skimlinks returned a commission without a valid ID")
		}
		commissionID := commission.CommissionID.String()
		if _, duplicate := commissionIDs[commissionID]; duplicate {
			return platformContractError(operation, "Skimlinks returned a duplicate commission ID")
		}
		commissionIDs[commissionID] = struct{}{}
		returnedPublisherID, validPublisherID := positiveExactID(commission.PublisherID)
		returnedDomainID, validDomainID := positiveExactID(commission.PublisherDomainID)
		advertiserID, validAdvertiserID := positiveExactID(commission.MerchantDetails.AdvertiserID)
		merchantID, validMerchantID := positiveExactID(commission.MerchantDetails.MerchantID)
		if !validMerchantID {
			merchantID, validMerchantID = positiveExactID(commission.MerchantDetails.ID)
		}
		if !validPublisherID || returnedPublisherID != publisherID || !validDomainID ||
			!validAdvertiserID || !validMerchantID {
			return platformContractError(operation, "Skimlinks returned a commission with invalid identity fields")
		}
		if input.DomainID > 0 && returnedDomainID != input.DomainID ||
			input.AdvertiserID > 0 && advertiserID != input.AdvertiserID ||
			input.MerchantID > 0 && merchantID != input.MerchantID ||
			input.CommissionID != "" && commissionID != input.CommissionID ||
			input.CustomID != "" && commission.ClickDetails.CustomID != input.CustomID ||
			input.Status != "" && commission.TransactionDetails.Status != string(input.Status) ||
			input.CommissionType != "" && commission.TransactionDetails.Basket.CommissionType != string(input.CommissionType) {
			return platformContractError(operation, "Skimlinks returned a commission outside the requested filters")
		}
	}
	return nil
}

func validatePerformanceReportResponse(
	operation string,
	response PerformanceReportResponse,
	input PerformanceReportRequest,
) error {
	limit := input.Limit
	if limit == 0 {
		limit = 30
	}
	if response.Reports == nil || response.Count < int64(len(response.Reports)) || len(response.Reports) > limit {
		return platformContractError(operation, "Skimlinks returned invalid performance-report collection metadata")
	}
	return nil
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Skimlinks GET workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "the implemented Skimlinks endpoints do not define field selection")
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Skimlinks does not document a caller request-ID header")
	}
	return resolved, nil
}

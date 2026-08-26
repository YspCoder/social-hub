package partnerize

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumFilterValues    = 1000
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

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validPathSegment(value string, maximum int) bool {
	return validOpaque(value, maximum) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#")
}

func validBasicUsername(value string) bool {
	return validOpaque(value, 4096) && !strings.Contains(value, ":")
}

func validYesNo(value YesNo) bool {
	return value == "" || value == Yes || value == No
}

func validListCampaigns(input ListCampaignsRequest) bool {
	return input.Status == CampaignApproved || input.Status == CampaignPending || input.Status == CampaignRejected
}

func validListCreatives(input ListCreativesRequest) bool {
	if !validPathSegment(input.CampaignID, 256) || !validYesNo(input.Active) ||
		!validOptionalOpaque(input.Tags, 4096) || len(input.CreativeTypeIDs) > maximumFilterValues {
		return false
	}
	for _, identifier := range input.CreativeTypeIDs {
		if !validOpaque(identifier, 256) {
			return false
		}
	}
	return true
}

func validDestinationURL(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 8192) || strings.Contains(strings.ToLower(value), "%00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (strings.EqualFold(parsed.Scheme, "https") || strings.EqualFold(parsed.Scheme, "http")) && parsed.Host != "" && parsed.User == nil
}

func validCreateTrackingLink(input CreateTrackingLinkRequest) bool {
	if !validOpaque(input.CampaignID, 256) || !validOptionalOpaque(input.Description, 8192) ||
		!validDestinationURL(input.DestinationURL) || len(input.Params) > maximumFilterValues {
		return false
	}
	for _, parameter := range input.Params {
		if !validOptionalOpaque(parameter.Key, 256) || !validOptionalOpaque(parameter.Value, 8192) {
			return false
		}
	}
	return true
}

func validCurrency(value Currency) bool {
	switch value {
	case "", CurrencyGBP, CurrencyUSD, CurrencyEUR, CurrencyJPY:
		return true
	default:
		return false
	}
}

func validConversionPivot(value ConversionPivot) bool {
	switch value {
	case "", ConversionPivotCampaign, ConversionPivotProduct, ConversionPivotPublisherReference:
		return true
	default:
		return false
	}
}

func validConversionStatus(value ConversionStatus) bool {
	return value == ConversionPending || value == ConversionApproved || value == ConversionRejected
}

func validListConversions(input ListConversionsRequest) bool {
	if input.StartDate.IsZero() || (!input.EndDate.IsZero() && !input.EndDate.After(input.StartDate)) ||
		!validOptionalOpaque(input.TextDate, 256) || !validOptionalOpaque(input.Timezone, 256) ||
		!validCurrency(input.Currency) || !validOptionalOpaque(input.DateType, 256) ||
		!validConversionPivot(input.Pivot) || input.Limit < 0 || input.Limit > 300 || input.CursorID < 0 || input.Offset < 0 ||
		(input.CursorID > 0 && (input.Offset > 0 || input.Limit == 0)) || len(input.PivotValues) > maximumFilterValues ||
		len(input.Statuses) > maximumFilterValues {
		return false
	}
	if (input.Pivot == "") != (len(input.PivotValues) == 0) {
		return false
	}
	for _, value := range input.PivotValues {
		if !validOpaque(value, 1024) {
			return false
		}
	}
	for _, status := range input.Statuses {
		if !validConversionStatus(status) {
			return false
		}
	}
	return true
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "the implemented Partnerize operations do not document idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "the implemented Partnerize operations do not define field selection")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func validatePartnerResponse(operation string, response PartnerResponse, expectedPublisherID string) error {
	if response.Partner.PublisherID != expectedPublisherID {
		return platformContractError(operation, "Partnerize returned a partner that does not match the configured publisher")
	}
	return nil
}

func validateCampaignsResponse(operation string, response CampaignsResponse) error {
	if response.Campaigns == nil {
		return platformContractError(operation, "Partnerize omitted the campaigns collection")
	}
	seen := make(map[string]struct{}, len(response.Campaigns))
	for _, wrapper := range response.Campaigns {
		identifier := wrapper.Campaign.CampaignID
		if !validOpaque(identifier, maximumProviderIDBytes) {
			return platformContractError(operation, "Partnerize returned a campaign without a valid ID")
		}
		if _, found := seen[identifier]; found {
			return platformContractError(operation, "Partnerize returned duplicate campaign IDs")
		}
		seen[identifier] = struct{}{}
	}
	return nil
}

func validateCreativesResponse(operation string, response CreativesResponse, expectedCampaignID string) error {
	if response.Creatives == nil {
		return platformContractError(operation, "Partnerize omitted the creatives collection")
	}
	seen := make(map[string]struct{}, len(response.Creatives))
	for _, wrapper := range response.Creatives {
		creative := wrapper.Creative
		if !validOpaque(creative.CreativeID, maximumProviderIDBytes) {
			return platformContractError(operation, "Partnerize returned a creative without a valid ID")
		}
		if creative.CampaignID != expectedCampaignID {
			return platformContractError(operation, "Partnerize returned a creative from a different campaign")
		}
		if _, found := seen[creative.CreativeID]; found {
			return platformContractError(operation, "Partnerize returned duplicate creative IDs")
		}
		seen[creative.CreativeID] = struct{}{}
	}
	return nil
}

func validateTrackingLinkResponse(operation string, response TrackingLinkResponse, expectedCampaignID string) error {
	if !validOpaque(response.Link.ID, maximumProviderIDBytes) {
		return platformContractError(operation, "Partnerize returned a tracking link without a valid ID")
	}
	if response.Link.CampaignID != expectedCampaignID {
		return platformContractError(operation, "Partnerize returned a tracking link for a different campaign")
	}
	if response.Link.TrackingURL == "" || !validDestinationURL(response.Link.TrackingURL) {
		return platformContractError(operation, "Partnerize returned an invalid tracking URL")
	}
	return nil
}

func validateConversionsResponse(operation string, response ConversionsResponse, expectedPublisherID string) error {
	if response.Conversions == nil {
		return platformContractError(operation, "Partnerize omitted the conversions collection")
	}
	seen := make(map[string]struct{}, len(response.Conversions))
	for _, wrapper := range response.Conversions {
		conversion := wrapper.Conversion
		if !validOpaque(conversion.ConversionID, maximumProviderIDBytes) {
			return platformContractError(operation, "Partnerize returned a conversion without a valid ID")
		}
		if conversion.PublisherID != expectedPublisherID {
			return platformContractError(operation, "Partnerize returned a conversion for a different publisher")
		}
		if _, found := seen[conversion.ConversionID]; found {
			return platformContractError(operation, "Partnerize returned duplicate conversion IDs")
		}
		seen[conversion.ConversionID] = struct{}{}
	}
	return nil
}

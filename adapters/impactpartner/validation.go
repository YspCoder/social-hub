package impactpartner

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maximumProviderStringRunes = 255
	maximumProviderIDBytes     = 4096
	maximumActionWindow        = 45 * 24 * time.Hour
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (strings.EqualFold(parsed.Scheme, "https") || strings.EqualFold(parsed.Scheme, "http")) && parsed.Host != "" &&
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

func validOptionalProviderString(value string) bool {
	return value == "" || (validOpaque(value, 4*maximumProviderStringRunes) && utf8.RuneCountInString(value) <= maximumProviderStringRunes)
}

func validPathSegment(value string, maximum int) bool {
	return validOpaque(value, maximum) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#")
}

func validDeepLink(value string) bool {
	if value == "" {
		return true
	}
	if !validOpaque(value, 4096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && (strings.EqualFold(parsed.Scheme, "https") || strings.EqualFold(parsed.Scheme, "http")) && parsed.Host != "" && parsed.User == nil
}

func validListPrograms(input ListProgramsRequest) bool {
	return input.InsertionOrderStatus == "" || input.InsertionOrderStatus == InsertionOrderActive || input.InsertionOrderStatus == InsertionOrderExpired
}

func validSearchCatalogItems(input SearchCatalogItemsRequest) bool {
	return validOptionalProviderString(input.Keyword) && input.PageSize >= 0 && input.Page >= 0
}

func validGetCatalogItem(input GetCatalogItemRequest) bool {
	return validPathSegment(input.CatalogID, maximumProviderStringRunes) && validPathSegment(input.ItemID, maximumProviderStringRunes)
}

func validCreateTrackingLink(input CreateTrackingLinkRequest) bool {
	if !validPathSegment(input.ProgramID, maximumProviderStringRunes) ||
		(input.Type != "" && input.Type != TrackingLinkRegular && input.Type != TrackingLinkVanity) ||
		!validDeepLink(input.DeepLink) {
		return false
	}
	for _, value := range []string{
		input.CustomPath, input.AdID, input.MediaPartnerPropertyID,
		input.SubID1, input.SubID2, input.SubID3, input.SharedID,
	} {
		if !validOptionalProviderString(value) {
			return false
		}
	}
	return true
}

func validListActions(input ListActionsRequest, now time.Time) bool {
	if input.CampaignID < 0 || input.CampaignID > 9_999_999_999_999_999 || input.Page < 0 || input.PageSize < 0 {
		return false
	}
	if input.State != "" && input.State != ActionPending && input.State != ActionApproved && input.State != ActionReversed {
		return false
	}
	// The v16 OAS permits one-sided ActionDate filters. When both are present,
	// apply the stricter current documentation window.
	if !input.ActionDateStart.IsZero() && !input.ActionDateEnd.IsZero() && !validDateWindow(input.ActionDateStart, input.ActionDateEnd) {
		return false
	}
	if !validRequiredDatePair(input.StartDate, input.EndDate) || !validRequiredDatePair(input.LockingDateStart, input.LockingDateEnd) {
		return false
	}
	if !input.StartDate.IsZero() && input.StartDate.Before(now.AddDate(-3, 0, 0)) {
		return false
	}
	return true
}

func validRequiredDatePair(start, end time.Time) bool {
	if start.IsZero() != end.IsZero() {
		return false
	}
	return start.IsZero() || validDateWindow(start, end)
}

func validDateWindow(start, end time.Time) bool {
	return !end.Before(start) && end.Sub(start) <= maximumActionWindow
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "impact.com Partner API v16 does not define idempotency keys for these workflows")
	}
	if len(resolved.Fields) > 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "these Partner API endpoints do not define a field-selection parameter")
	}
	if resolved.RequestID != "" && !validOpaque(resolved.RequestID, 256) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func validateProgramsResponse(operation string, response ProgramsResponse) error {
	if response.Programs == nil {
		return platformContractError(operation, "impact.com omitted the Campaigns collection")
	}
	seen := make(map[string]struct{}, len(response.Programs))
	for _, program := range response.Programs {
		if !validOpaque(program.CampaignID, maximumProviderIDBytes) {
			return platformContractError(operation, "impact.com returned a program without a valid campaign ID")
		}
		if _, found := seen[program.CampaignID]; found {
			return platformContractError(operation, "impact.com returned duplicate campaign IDs")
		}
		seen[program.CampaignID] = struct{}{}
	}
	return nil
}

func validateCatalogItemsResponse(operation string, response CatalogItemsResponse) error {
	if response.Items == nil {
		return platformContractError(operation, "impact.com omitted the Items collection")
	}
	seen := make(map[string]struct{}, len(response.Items))
	for _, item := range response.Items {
		if !validOpaque(item.ID, maximumProviderIDBytes) || !validOpaque(item.CatalogID, maximumProviderIDBytes) {
			return platformContractError(operation, "impact.com returned a catalog item without valid item and catalog IDs")
		}
		if _, found := seen[item.ID]; found {
			return platformContractError(operation, "impact.com returned duplicate catalog item IDs")
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func validateCatalogItemResponse(operation string, response CatalogItem, expectedCatalogID, expectedItemID string) error {
	if response.CatalogID != expectedCatalogID || response.ID != expectedItemID {
		return platformContractError(operation, "impact.com returned a catalog item that does not match the request")
	}
	return nil
}

func validateTrackingLinkResponse(operation string, response TrackingLink) error {
	if response.TrackingURL == "" || !validDeepLink(response.TrackingURL) {
		return platformContractError(operation, "impact.com returned an invalid tracking URL")
	}
	return nil
}

func validateActionsResponse(operation string, response ActionsResponse) error {
	if response.Actions == nil {
		return platformContractError(operation, "impact.com omitted the Actions collection")
	}
	seen := make(map[string]struct{}, len(response.Actions))
	for _, action := range response.Actions {
		if !validOpaque(action.ID, maximumProviderIDBytes) {
			return platformContractError(operation, "impact.com returned an action without a valid ID")
		}
		if _, found := seen[action.ID]; found {
			return platformContractError(operation, "impact.com returned duplicate action IDs")
		}
		seen[action.ID] = struct{}{}
	}
	return nil
}

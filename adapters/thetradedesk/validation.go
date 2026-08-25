package thetradedesk

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const maxStructuredCollectionItems = 4096

func resolveCallContext(ctx context.Context, operation string, options []socialhub.CallOption) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, invalidArgument(operation, "context must not be nil")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, invalidArgument(operation, "call options are invalid")
	}
	if resolved.RequestID != "" {
		return nil, nil, invalidArgument(operation, "caller request IDs are not part of the Platform API contract")
	}
	if resolved.IdempotencyKey != "" {
		return nil, nil, invalidArgument(operation, "the Platform API does not document an idempotency-key contract")
	}
	if len(resolved.Fields) > 0 {
		return nil, nil, invalidArgument(operation, "field selection is fixed by the typed REST operation")
	}
	if resolved.Timeout < 0 {
		return nil, nil, invalidArgument(operation, "call timeout must not be negative")
	}
	if resolved.Timeout > 0 {
		callContext, cancel := context.WithTimeout(ctx, resolved.Timeout)
		return callContext, cancel, nil
	}
	return ctx, func() {}, nil
}

func validEndpoint(value string) bool {
	return value == productionBaseURL || value == sandboxBaseURL
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

func validID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validText(value string, maximum int) bool {
	return value != "" && validOptionalText(value, maximum) && value == strings.TrimSpace(value)
}

func validOptionalText(value string, maximum int) bool {
	if value == "" {
		return true
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validDateTime(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validMoney(value Money, allowZero bool) bool {
	if len(value.CurrencyCode) != 3 {
		return false
	}
	for _, character := range value.CurrencyCode {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	raw := string(value.Amount)
	if len(raw) == 0 || len(raw) > 64 || !json.Valid([]byte(raw)) {
		return false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return false
	}
	return allowZero || parsed > 0
}

func validAvailability(value Availability) bool {
	return value == AvailabilityAvailable || value == AvailabilityArchived
}

func validCampaignType(value CampaignType) bool {
	return value == CampaignTypeStandard || value == CampaignTypeProgrammaticGuaranteed
}

func validCampaignVersion(value CampaignVersion) bool { return value == CampaignVersionKokai }

func validBudgetingVersion(value CampaignBudgetingVersion) bool {
	return value == CampaignBudgetingSolimar || value == CampaignBudgetingKokai
}

func validPacingMode(value PacingMode) bool {
	switch value {
	case PacingOff, PacingToEndOfFlight, PacingAhead, PacingAsSoonAsPossible:
		return true
	default:
		return false
	}
}

func validChannel(value Channel) bool {
	switch value {
	case ChannelDisplay, ChannelVideo, ChannelAudio, ChannelTV, ChannelNativeDisplay,
		ChannelNativeVideo, ChannelDigitalOutOfHome:
		return true
	default:
		return false
	}
}

func validNewBuyerTarget(value NewBuyerTargetType) bool {
	return value == NewBuyerTargetFeatured || value == NewBuyerTargetTotal
}

func validCampaignGoal(value CampaignGoal) bool {
	fields := 0
	for _, money := range []*Money{
		value.CPAInAdvertiserCurrency, value.CPCInAdvertiserCurrency,
		value.CPCVInAdvertiserCurrency, value.VCPMInAdvertiserCurrency,
	} {
		if money != nil {
			fields++
			if !validMoney(*money, false) {
				return false
			}
		}
	}
	for _, target := range []*float64{
		value.CTRInPercent, value.MiaozhenOTPInPercent, value.NielsenOTPInPercent,
		value.ReturnOnAdSpendPercent, value.VCRInPercent, value.ViewabilityInPercent,
	} {
		if target != nil {
			fields++
			if !validFiniteNonnegative(target) {
				return false
			}
		}
	}
	for _, enabled := range []bool{value.MaximizeLTVReach, value.MaximizeReach, value.MaximizeConversionRevenue} {
		if enabled {
			fields++
		}
	}
	if value.NewBuyerTargetValue != "" {
		fields++
		if !validNewBuyerTarget(value.NewBuyerTargetValue) {
			return false
		}
	}
	return fields == 1
}

func validSortField(value CampaignSortField) bool {
	switch value {
	case SortCampaignName, SortCampaignDescription, SortCampaignBudget,
		SortCampaignBudgetInImpressions, SortCampaignDailyBudget,
		SortCampaignDailyBudgetInImpressions, SortCampaignCreatedAtUTC,
		SortCampaignLastUpdatedAtUTC:
		return true
	default:
		return false
	}
}

func validFiniteNonnegative(value *float64) bool {
	return value == nil || *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func validConversionColumnInputs(values []ConversionReportingColumnInput) bool {
	if len(values) > maxStructuredCollectionItems {
		return false
	}
	seenPairs := make(map[string]struct{}, len(values))
	seenColumns := make(map[int32]struct{}, len(values))
	withAttributionModel := 0
	for _, value := range values {
		if !validID(value.TrackingTagID) || !validFiniteNonnegative(value.Weight) {
			return false
		}
		model := ""
		if value.CrossDeviceAttributionModelID != nil {
			if !validID(*value.CrossDeviceAttributionModelID) {
				return false
			}
			model = *value.CrossDeviceAttributionModelID
			withAttributionModel++
		}
		pair := value.TrackingTagID + "\x00" + model
		if _, exists := seenPairs[pair]; exists {
			return false
		}
		seenPairs[pair] = struct{}{}
		if value.ReportingColumnID <= 0 {
			return false
		}
		if _, exists := seenColumns[value.ReportingColumnID]; exists {
			return false
		}
		seenColumns[value.ReportingColumnID] = struct{}{}
		if value.CustomROAS != nil && (!validFiniteNonnegative(value.CustomROAS.ClickWeight) ||
			!validFiniteNonnegative(value.CustomROAS.ViewthroughWeight) || !validFiniteNonnegative(value.CustomROAS.Weight)) {
			return false
		}
	}
	return withAttributionModel == 0 || withAttributionModel == len(values)
}

func validConversionColumnResponses(values []ConversionReportingColumn) bool {
	seenPairs := make(map[string]struct{}, len(values))
	for _, value := range values {
		input := value.ConversionReportingColumnInput
		if input.TrackingTagID != "" && !validID(input.TrackingTagID) || !validFiniteNonnegative(input.Weight) ||
			value.TrackingTagName != nil && !validOptionalText(*value.TrackingTagName, 512) {
			return false
		}
		model := ""
		if input.CrossDeviceAttributionModelID != nil {
			if !validID(*input.CrossDeviceAttributionModelID) {
				return false
			}
			model = *input.CrossDeviceAttributionModelID
		}
		if input.TrackingTagID != "" {
			pair := input.TrackingTagID + "\x00" + model
			if _, exists := seenPairs[pair]; exists {
				return false
			}
			seenPairs[pair] = struct{}{}
		}
		if input.ReportingColumnID < 0 || input.CustomROAS != nil &&
			(!validFiniteNonnegative(input.CustomROAS.ClickWeight) ||
				!validFiniteNonnegative(input.CustomROAS.ViewthroughWeight) ||
				!validFiniteNonnegative(input.CustomROAS.Weight)) {
			return false
		}
	}
	return true
}

func validCampaignQuery(input CampaignQuery) bool {
	if input.PageStartIndex < 0 || input.PageSize != nil && (*input.PageSize < 0 || *input.PageSize > 1000) ||
		len(input.Availabilities) > 2 || len(input.SearchTerms) > maxStructuredCollectionItems || len(input.SortFields) > 8 {
		return false
	}
	seenAvailability := make(map[Availability]struct{}, len(input.Availabilities))
	for _, value := range input.Availabilities {
		if !validAvailability(value) {
			return false
		}
		if _, exists := seenAvailability[value]; exists {
			return false
		}
		seenAvailability[value] = struct{}{}
	}
	for _, value := range input.SearchTerms {
		if !validText(value, maxRequestBytes) {
			return false
		}
	}
	seenSort := make(map[CampaignSortField]struct{}, len(input.SortFields))
	for _, value := range input.SortFields {
		if !validSortField(value.Field) {
			return false
		}
		if _, exists := seenSort[value.Field]; exists {
			return false
		}
		seenSort[value.Field] = struct{}{}
	}
	return true
}

func validCreateCampaign(input CreateCampaignRequest) bool {
	if !validText(input.Name, 256) || !validOptionalText(input.Description, 2048) ||
		!validConversionColumnInputs(input.ConversionReportingColumns) || !validCampaignGoal(input.PrimaryGoal) ||
		!validDateTime(input.StartDate) {
		return false
	}
	if (input.Budget == nil) == (input.BudgetInImpressions == nil) {
		return false
	}
	if input.Budget != nil && !validMoney(*input.Budget, false) ||
		input.BudgetInImpressions != nil && *input.BudgetInImpressions <= 0 ||
		input.DailyBudget != nil && !validMoney(*input.DailyBudget, false) ||
		input.DailyBudgetInImpressions != nil && *input.DailyBudgetInImpressions <= 0 ||
		input.DailyBudget != nil && input.DailyBudgetInImpressions != nil {
		return false
	}
	if input.EndDate != nil {
		if !validDateTime(*input.EndDate) {
			return false
		}
		start, _ := time.Parse(time.RFC3339Nano, input.StartDate)
		end, _ := time.Parse(time.RFC3339Nano, *input.EndDate)
		if end.Before(start) {
			return false
		}
	}
	if input.PacingMode != "" && !validPacingMode(input.PacingMode) ||
		input.PacingMode == PacingOff && input.DailyBudget == nil && input.DailyBudgetInImpressions == nil ||
		input.Type != "" && !validCampaignType(input.Type) ||
		input.Version != CampaignVersionKokai ||
		input.BudgetingVersion != "" && !validBudgetingVersion(input.BudgetingVersion) ||
		!validChannel(input.PrimaryChannel) ||
		input.TimeZone != "" && !validText(input.TimeZone, 128) ||
		input.SeedID != "" && (!validID(input.SeedID) || input.Version != CampaignVersionKokai) ||
		input.PurchaseOrderNumber != nil && !validOptionalText(*input.PurchaseOrderNumber, maxRequestBytes) {
		return false
	}
	return true
}

func validUpdateCampaign(input UpdateCampaignRequest) bool {
	if input.Budget != nil && input.BudgetInImpressions != nil ||
		input.DailyBudget != nil && input.DailyBudgetInImpressions != nil {
		return false
	}
	fields := 0
	if input.Name != nil {
		fields++
		if !validText(*input.Name, 256) {
			return false
		}
	}
	if input.Description != nil {
		fields++
		if !validOptionalText(*input.Description, 2048) {
			return false
		}
	}
	if input.Availability != nil {
		fields++
		if !validAvailability(*input.Availability) {
			return false
		}
	}
	if input.Budget != nil {
		fields++
		if !validMoney(*input.Budget, false) {
			return false
		}
	}
	if input.BudgetInImpressions != nil || input.ClearBudgetInImpressions {
		fields++
		if input.BudgetInImpressions != nil && (input.ClearBudgetInImpressions || *input.BudgetInImpressions <= 0) {
			return false
		}
	}
	if input.DailyBudget != nil || input.ClearDailyBudget {
		fields++
		if input.DailyBudget != nil && (input.ClearDailyBudget || !validMoney(*input.DailyBudget, false)) {
			return false
		}
	}
	if input.DailyBudgetInImpressions != nil || input.ClearDailyBudgetInImpressions {
		fields++
		if input.DailyBudgetInImpressions != nil && (input.ClearDailyBudgetInImpressions || *input.DailyBudgetInImpressions <= 0) {
			return false
		}
	}
	if input.StartDate != nil {
		fields++
		if !validDateTime(*input.StartDate) {
			return false
		}
	}
	if input.EndDate != nil || input.ClearEndDate {
		fields++
		if input.EndDate != nil && (input.ClearEndDate || !validDateTime(*input.EndDate)) {
			return false
		}
	}
	if input.StartDate != nil && input.EndDate != nil {
		start, _ := time.Parse(time.RFC3339Nano, *input.StartDate)
		end, _ := time.Parse(time.RFC3339Nano, *input.EndDate)
		if end.Before(start) {
			return false
		}
	}
	if input.TimeZone != nil {
		fields++
		if !validText(*input.TimeZone, 128) {
			return false
		}
	}
	if input.PacingMode != nil {
		fields++
		if !validPacingMode(*input.PacingMode) {
			return false
		}
	}
	if input.PrimaryChannel != nil {
		fields++
		if !validChannel(*input.PrimaryChannel) {
			return false
		}
	}
	if input.SeedID != nil {
		fields++
		if !validID(*input.SeedID) {
			return false
		}
	}
	if input.PurchaseOrderNumber != nil || input.ClearPurchaseOrderNumber {
		fields++
		if input.PurchaseOrderNumber != nil && (input.ClearPurchaseOrderNumber || !validOptionalText(*input.PurchaseOrderNumber, maxRequestBytes)) {
			return false
		}
	}
	if input.ConversionReportingColumns != nil {
		fields++
		if *input.ConversionReportingColumns != nil && !validConversionColumnInputs(*input.ConversionReportingColumns) {
			return false
		}
	}
	return fields > 0
}

func validCampaignResponse(value Campaign) bool {
	if !validID(value.ID) || !validID(value.AdvertiserID) || !validText(value.Name, 256) ||
		value.Availability != "" && !validAvailability(value.Availability) ||
		value.Budget != nil && !validMoney(*value.Budget, true) ||
		value.BudgetInImpressions != nil && *value.BudgetInImpressions < 0 ||
		value.DailyBudget != nil && !validMoney(*value.DailyBudget, true) ||
		value.DailyBudgetInImpressions != nil && *value.DailyBudgetInImpressions < 0 ||
		value.StartDate != "" && !validDateTime(value.StartDate) ||
		value.EndDate != nil && !validDateTime(*value.EndDate) ||
		value.PacingMode != "" && !validPacingMode(value.PacingMode) ||
		value.Type != "" && !validCampaignType(value.Type) ||
		value.Version != "" && !validCampaignVersion(value.Version) ||
		value.BudgetingVersion != "" && !validBudgetingVersion(value.BudgetingVersion) ||
		value.PrimaryChannel != "" && !validChannel(value.PrimaryChannel) ||
		!validConversionColumnResponses(value.ConversionReportingColumns) {
		return false
	}
	for _, timestamp := range []*string{value.CreatedAtUTC, value.LastUpdatedAtUTC} {
		if timestamp != nil && !validDateTime(*timestamp) {
			return false
		}
	}
	return true
}

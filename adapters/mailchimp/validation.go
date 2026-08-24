package mailchimp

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
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

func validAPIKey(value string) bool {
	if len(value) < 16 || !validOpaque(value, 512) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func validDataCenter(value string) bool {
	if len(value) < 3 || len(value) > 5 || !strings.HasPrefix(value, "us") {
		return false
	}
	digits := value[2:]
	if digits[0] == '0' {
		return false
	}
	for _, character := range digits {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func dataCenterFromAPIKey(apiKey string) (string, bool) {
	separator := strings.LastIndexByte(apiKey, '-')
	if separator <= 0 || separator == len(apiKey)-1 {
		return "", false
	}
	dataCenter := apiKey[separator+1:]
	return dataCenter, validDataCenter(dataCenter)
}

func resolveDataCenter(apiKey, configured string) (string, error) {
	fromKey, hasSuffix := dataCenterFromAPIKey(apiKey)
	if configured != "" {
		if !validDataCenter(configured) {
			return "", fmt.Errorf("configured data center is invalid")
		}
		if hasSuffix && fromKey != configured {
			return "", fmt.Errorf("configured data center does not match the API-key suffix")
		}
		return configured, nil
	}
	if !hasSuffix {
		return "", fmt.Errorf("API key must have a valid data-center suffix or account.settings.data_center must be set")
	}
	return fromKey, nil
}

func validResourceID(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validPagination(value Pagination) bool {
	return value.Count >= 0 && value.Count <= 1000 && value.Offset >= 0
}

func parseOptionalTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	if !validOpaque(value, 128) {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func validTimeRange(since, before string) bool {
	sinceTime, validSince := parseOptionalTime(since)
	beforeTime, validBefore := parseOptionalTime(before)
	if !validSince || !validBefore {
		return false
	}
	return since == "" || before == "" || !beforeTime.Before(sinceTime)
}

func validSortDirection(value SortDirection) bool {
	return value == "" || value == SortAscending || value == SortDescending
}

func validCampaignType(value CampaignType) bool {
	switch value {
	case "", CampaignTypeRegular, CampaignTypePlaintext, CampaignTypeABSplit, CampaignTypeRSS, CampaignTypeVariate:
		return true
	default:
		return false
	}
}

func validCampaignFilterStatus(value CampaignStatus) bool {
	switch value {
	case "", CampaignStatusSave, CampaignStatusPaused, CampaignStatusSchedule, CampaignStatusSending, CampaignStatusSent:
		return true
	default:
		return false
	}
}

func validCampaignResponseStatus(value CampaignStatus) bool {
	if validCampaignFilterStatus(value) {
		return true
	}
	return value == CampaignStatusCanceled || value == CampaignStatusCanceling || value == CampaignStatusArchived
}

func validListRequest(value ListListsRequest) bool {
	if !validPagination(value.Page) || !validTimeRange(value.SinceDateCreated, value.BeforeDateCreated) ||
		!validTimeRange(value.SinceCampaignLastSent, value.BeforeCampaignLastSent) || !validSortDirection(value.SortDirection) {
		return false
	}
	if value.SortField != "" && value.SortField != ListSortDateCreated {
		return false
	}
	return value.SortDirection == "" || value.SortField != ""
}

func validCampaignRequest(value ListCampaignsRequest) bool {
	if !validPagination(value.Page) || !validCampaignType(value.Type) || !validCampaignFilterStatus(value.Status) ||
		!validTimeRange(value.SinceSendTime, value.BeforeSendTime) || !validTimeRange(value.SinceCreateTime, value.BeforeCreateTime) ||
		!validOptionalResourceID(value.ListID) || !validOptionalResourceID(value.FolderID) || !validSortDirection(value.SortDirection) {
		return false
	}
	if value.SortField != "" && value.SortField != CampaignSortCreateTime && value.SortField != CampaignSortSendTime {
		return false
	}
	return value.SortDirection == "" || value.SortField != ""
}

func validReportRequest(value ListReportsRequest) bool {
	return validPagination(value.Page) && validCampaignType(value.Type) && validTimeRange(value.SinceSendTime, value.BeforeSendTime)
}

func validOptionalResourceID(value string) bool {
	return value == "" || validResourceID(value)
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Mailchimp Marketing API does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Mailchimp operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed to the typed non-PII Mailchimp projection")
	}
	return nil
}

func validAudience(value Audience, expectedID string) bool {
	if !validResourceID(value.ID) || len(value.Raw) == 0 || value.Stats.TotalContacts < 0 || expectedID != "" && value.ID != expectedID {
		return false
	}
	for _, channel := range value.EnabledChannels {
		if !validOpaque(channel, 128) {
			return false
		}
	}
	return true
}

func validAudiencePage(value AudiencePage) bool {
	if value.TotalItems < 0 || value.TotalItems < len(value.Audiences) || len(value.Raw) == 0 {
		return false
	}
	for _, audience := range value.Audiences {
		if !validAudience(audience, "") {
			return false
		}
	}
	return true
}

func validList(value List, expectedID string) bool {
	return validResourceID(value.ID) && len(value.Raw) > 0 && (expectedID == "" || value.ID == expectedID) &&
		value.WebID >= 0 && value.ListRating >= 0 && value.Stats.MemberCount >= 0 && value.Stats.UnsubscribeCount >= 0 &&
		value.Stats.CleanedCount >= 0 && value.Stats.CampaignCount >= 0
}

func validListPage(value ListPage) bool {
	if value.TotalItems < 0 || value.TotalItems < len(value.Lists) || len(value.Raw) == 0 {
		return false
	}
	for _, list := range value.Lists {
		if !validList(list, "") {
			return false
		}
	}
	return true
}

func validDeliveryStatus(value DeliveryStatus) bool {
	if value.EmailsSent < 0 || value.EmailsCanceled < 0 {
		return false
	}
	switch value.Status {
	case "", DeliveryStateDelivering, DeliveryStateDelivered, DeliveryStateCanceling, DeliveryStateCanceled:
		return true
	default:
		return false
	}
}

func validCampaign(value Campaign, expectedID string) bool {
	return validResourceID(value.ID) && len(value.Raw) > 0 && (expectedID == "" || value.ID == expectedID) &&
		validCampaignType(value.Type) && validCampaignResponseStatus(value.Status) && value.WebID >= 0 && value.EmailsSent >= 0 &&
		value.Recipients.RecipientCount >= 0 && value.ReportSummary.Opens >= 0 && value.ReportSummary.UniqueOpens >= 0 &&
		value.ReportSummary.Clicks >= 0 && value.ReportSummary.SubscriberClicks >= 0 && validDeliveryStatus(value.DeliveryStatus)
}

func validCampaignPage(value CampaignPage) bool {
	if value.TotalItems < 0 || value.TotalItems < len(value.Campaigns) || len(value.Raw) == 0 {
		return false
	}
	for _, campaign := range value.Campaigns {
		if !validCampaign(campaign, "") {
			return false
		}
	}
	return true
}

func validReport(value CampaignReport, expectedID string) bool {
	return validResourceID(value.ID) && len(value.Raw) > 0 && (expectedID == "" || value.ID == expectedID) &&
		value.EmailsSent >= 0 && value.AbuseReports >= 0 && value.Unsubscribed >= 0 && value.Bounces.HardBounces >= 0 &&
		value.Bounces.SoftBounces >= 0 && value.Bounces.SyntaxErrors >= 0 && value.Forwards.ForwardsCount >= 0 &&
		value.Forwards.ForwardsOpens >= 0 && value.Opens.OpensTotal >= 0 && value.Opens.UniqueOpens >= 0 &&
		value.Clicks.ClicksTotal >= 0 && value.Clicks.UniqueClicks >= 0 && value.Clicks.UniqueSubscriberClicks >= 0 &&
		validDeliveryStatus(value.DeliveryStatus)
}

func validReportPage(value ReportPage) bool {
	if value.TotalItems < 0 || value.TotalItems < len(value.Reports) || len(value.Raw) == 0 {
		return false
	}
	for _, report := range value.Reports {
		if !validReport(report, "") {
			return false
		}
	}
	return true
}

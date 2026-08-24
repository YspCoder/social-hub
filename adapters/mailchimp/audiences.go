package mailchimp

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	audienceFields           = "id,name,stats.total_contacts,enabled_channels"
	audienceCollectionFields = "audiences.id,audiences.name,audiences.stats.total_contacts,audiences.enabled_channels,total_items"
)

// ListAudiences returns non-PII audience metadata and complete contact counts.
func (client *Client) ListAudiences(ctx context.Context, input ListAudiencesRequest, options ...socialhub.CallOption) (AudiencePage, error) {
	const operation = "list_audiences"
	if !validPagination(input.Page) {
		return AudiencePage{}, invalidArgument(operation, "count must be between 0 and 1000 and offset must not be negative")
	}
	query := make(url.Values)
	setPagination(query, input.Page)
	query.Set("fields", audienceCollectionFields)
	var page AudiencePage
	meta, _, err := client.getJSON(ctx, operation, "/audiences", query, &page, options...)
	if err != nil {
		return AudiencePage{}, err
	}
	page.Page, page.Meta = input.Page, meta
	if !validAudiencePage(page) {
		return AudiencePage{}, platformContractError(operation, "Mailchimp returned an invalid audience page or total_items value")
	}
	return page, nil
}

// GetAudience returns non-PII metadata for one audience.
func (client *Client) GetAudience(ctx context.Context, input GetAudienceRequest, options ...socialhub.CallOption) (Audience, error) {
	const operation = "get_audience"
	if !validResourceID(input.AudienceID) {
		return Audience{}, invalidArgument(operation, "audience ID must be a safe bounded path segment")
	}
	query := url.Values{"fields": {audienceFields}}
	var audience Audience
	meta, _, err := client.getJSON(ctx, operation, "/audiences/"+input.AudienceID, query, &audience, options...)
	if err != nil {
		return Audience{}, err
	}
	audience.Meta = meta
	if !validAudience(audience, input.AudienceID) {
		return Audience{}, platformContractError(operation, "Mailchimp returned an absent or mismatched audience ID")
	}
	return audience, nil
}

func setPagination(query url.Values, value Pagination) {
	if value.Count > 0 {
		query.Set("count", strconv.Itoa(value.Count))
	}
	if value.Offset > 0 {
		query.Set("offset", strconv.Itoa(value.Offset))
	}
}

package mailchimp

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const (
	listFields           = "id,web_id,name,date_created,list_rating,email_type_option,visibility,double_optin,has_welcome,marketing_permissions,stats.member_count,stats.unsubscribe_count,stats.cleaned_count,stats.campaign_count,stats.campaign_last_sent,stats.open_rate,stats.click_rate"
	listCollectionFields = "lists.id,lists.web_id,lists.name,lists.date_created,lists.list_rating,lists.email_type_option,lists.visibility,lists.double_optin,lists.has_welcome,lists.marketing_permissions,lists.stats.member_count,lists.stats.unsubscribe_count,lists.stats.cleaned_count,lists.stats.campaign_count,lists.stats.campaign_last_sent,lists.stats.open_rate,lists.stats.click_rate,total_items"
)

// ListLists returns privacy-bounded metadata for legacy Mailchimp Lists.
func (client *Client) ListLists(ctx context.Context, input ListListsRequest, options ...socialhub.CallOption) (ListPage, error) {
	const operation = "list_lists"
	if !validListRequest(input) {
		return ListPage{}, invalidArgument(operation, "pagination, date ranges, sort, or ecommerce-store filter is invalid")
	}
	query := make(url.Values)
	setPagination(query, input.Page)
	setStringQuery(query, "since_date_created", input.SinceDateCreated)
	setStringQuery(query, "before_date_created", input.BeforeDateCreated)
	setStringQuery(query, "since_campaign_last_sent", input.SinceCampaignLastSent)
	setStringQuery(query, "before_campaign_last_sent", input.BeforeCampaignLastSent)
	setStringQuery(query, "sort_field", string(input.SortField))
	setStringQuery(query, "sort_dir", string(input.SortDirection))
	setBoolQuery(query, "has_ecommerce_store", input.HasEcommerceStore)
	query.Set("fields", listCollectionFields)
	var page ListPage
	meta, _, err := client.getJSON(ctx, operation, "/lists", query, &page, options...)
	if err != nil {
		return ListPage{}, err
	}
	page.Page, page.Meta = input.Page, meta
	if !validListPage(page) {
		return ListPage{}, platformContractError(operation, "Mailchimp returned an invalid list page or total_items value")
	}
	return page, nil
}

// GetList returns privacy-bounded metadata for one legacy Mailchimp List.
func (client *Client) GetList(ctx context.Context, input GetListRequest, options ...socialhub.CallOption) (List, error) {
	const operation = "get_list"
	if !validResourceID(input.ListID) {
		return List{}, invalidArgument(operation, "list ID must be a safe bounded path segment")
	}
	query := url.Values{"fields": {listFields}}
	var list List
	meta, _, err := client.getJSON(ctx, operation, "/lists/"+input.ListID, query, &list, options...)
	if err != nil {
		return List{}, err
	}
	list.Meta = meta
	if !validList(list, input.ListID) {
		return List{}, platformContractError(operation, "Mailchimp returned an absent or mismatched list ID")
	}
	return list, nil
}

func setStringQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setBoolQuery(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

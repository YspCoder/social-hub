package mailchimp

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	campaignFields           = "id,web_id,parent_campaign_id,type,create_time,archive_url,status,emails_sent,send_time,content_type,resendable,recipients.list_id,recipients.list_is_active,recipients.list_name,recipients.recipient_count,settings.subject_line,settings.preview_text,settings.title,settings.folder_id,tracking.opens,tracking.html_clicks,tracking.text_clicks,tracking.goal_tracking,tracking.ecomm360,report_summary.opens,report_summary.unique_opens,report_summary.open_rate,report_summary.clicks,report_summary.subscriber_clicks,report_summary.click_rate,report_summary.ecommerce.total_orders,report_summary.ecommerce.total_spent,report_summary.ecommerce.total_revenue,delivery_status.enabled,delivery_status.can_cancel,delivery_status.status,delivery_status.emails_sent,delivery_status.emails_canceled"
	campaignCollectionFields = "campaigns.id,campaigns.web_id,campaigns.parent_campaign_id,campaigns.type,campaigns.create_time,campaigns.archive_url,campaigns.status,campaigns.emails_sent,campaigns.send_time,campaigns.content_type,campaigns.resendable,campaigns.recipients.list_id,campaigns.recipients.list_is_active,campaigns.recipients.list_name,campaigns.recipients.recipient_count,campaigns.settings.subject_line,campaigns.settings.preview_text,campaigns.settings.title,campaigns.settings.folder_id,campaigns.tracking.opens,campaigns.tracking.html_clicks,campaigns.tracking.text_clicks,campaigns.tracking.goal_tracking,campaigns.tracking.ecomm360,campaigns.report_summary.opens,campaigns.report_summary.unique_opens,campaigns.report_summary.open_rate,campaigns.report_summary.clicks,campaigns.report_summary.subscriber_clicks,campaigns.report_summary.click_rate,campaigns.report_summary.ecommerce.total_orders,campaigns.report_summary.ecommerce.total_spent,campaigns.report_summary.ecommerce.total_revenue,campaigns.delivery_status.enabled,campaigns.delivery_status.can_cancel,campaigns.delivery_status.status,campaigns.delivery_status.emails_sent,campaigns.delivery_status.emails_canceled,total_items"
)

// ListCampaigns returns campaign metadata without content or member data.
func (client *Client) ListCampaigns(ctx context.Context, input ListCampaignsRequest, options ...socialhub.CallOption) (CampaignPage, error) {
	const operation = "list_campaigns"
	if !validCampaignRequest(input) {
		return CampaignPage{}, invalidArgument(operation, "pagination, type, status, dates, IDs, or sort is invalid")
	}
	query := make(url.Values)
	setPagination(query, input.Page)
	setStringQuery(query, "type", string(input.Type))
	setStringQuery(query, "status", string(input.Status))
	setStringQuery(query, "since_send_time", input.SinceSendTime)
	setStringQuery(query, "before_send_time", input.BeforeSendTime)
	setStringQuery(query, "since_create_time", input.SinceCreateTime)
	setStringQuery(query, "before_create_time", input.BeforeCreateTime)
	setStringQuery(query, "list_id", input.ListID)
	setStringQuery(query, "folder_id", input.FolderID)
	setStringQuery(query, "sort_field", string(input.SortField))
	setStringQuery(query, "sort_dir", string(input.SortDirection))
	query.Set("fields", campaignCollectionFields)
	var page CampaignPage
	meta, _, err := client.getJSON(ctx, operation, "/campaigns", query, &page, options...)
	if err != nil {
		return CampaignPage{}, err
	}
	page.Page, page.Meta = input.Page, meta
	if !validCampaignPage(page) {
		return CampaignPage{}, platformContractError(operation, "Mailchimp returned an invalid campaign page or total_items value")
	}
	return page, nil
}

// GetCampaign returns metadata for one campaign without content or member data.
func (client *Client) GetCampaign(ctx context.Context, input GetCampaignRequest, options ...socialhub.CallOption) (Campaign, error) {
	const operation = "get_campaign"
	if !validResourceID(input.CampaignID) {
		return Campaign{}, invalidArgument(operation, "campaign ID must be a safe bounded path segment")
	}
	query := url.Values{"fields": {campaignFields}}
	var campaign Campaign
	meta, _, err := client.getJSON(ctx, operation, "/campaigns/"+input.CampaignID, query, &campaign, options...)
	if err != nil {
		return Campaign{}, err
	}
	campaign.Meta = meta
	if !validCampaign(campaign, input.CampaignID) {
		return Campaign{}, platformContractError(operation, "Mailchimp returned an absent, mismatched, or invalid campaign")
	}
	return campaign, nil
}

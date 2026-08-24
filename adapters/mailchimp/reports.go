package mailchimp

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	reportFields           = "id,campaign_title,type,list_id,list_is_active,list_name,subject_line,preview_text,emails_sent,abuse_reports,unsubscribed,send_time,rss_last_send,bounces.hard_bounces,bounces.soft_bounces,bounces.syntax_errors,forwards.forwards_count,forwards.forwards_opens,opens.opens_total,opens.proxy_excluded_opens,opens.unique_opens,opens.proxy_excluded_unique_opens,opens.open_rate,opens.proxy_excluded_open_rate,opens.last_open,clicks.clicks_total,clicks.unique_clicks,clicks.unique_subscriber_clicks,clicks.click_rate,clicks.last_click,industry_stats.type,industry_stats.open_rate,industry_stats.click_rate,industry_stats.bounce_rate,industry_stats.unopen_rate,industry_stats.unsub_rate,industry_stats.abuse_rate,list_stats.sub_rate,list_stats.unsub_rate,list_stats.open_rate,list_stats.proxy_excluded_open_rate,list_stats.click_rate,ecommerce.total_orders,ecommerce.total_spent,ecommerce.total_revenue,ecommerce.currency_code,delivery_status.enabled,delivery_status.can_cancel,delivery_status.status,delivery_status.emails_sent,delivery_status.emails_canceled"
	reportCollectionFields = "reports.id,reports.campaign_title,reports.type,reports.list_id,reports.list_is_active,reports.list_name,reports.subject_line,reports.preview_text,reports.emails_sent,reports.abuse_reports,reports.unsubscribed,reports.send_time,reports.rss_last_send,reports.bounces.hard_bounces,reports.bounces.soft_bounces,reports.bounces.syntax_errors,reports.forwards.forwards_count,reports.forwards.forwards_opens,reports.opens.opens_total,reports.opens.proxy_excluded_opens,reports.opens.unique_opens,reports.opens.proxy_excluded_unique_opens,reports.opens.open_rate,reports.opens.proxy_excluded_open_rate,reports.opens.last_open,reports.clicks.clicks_total,reports.clicks.unique_clicks,reports.clicks.unique_subscriber_clicks,reports.clicks.click_rate,reports.clicks.last_click,reports.industry_stats.type,reports.industry_stats.open_rate,reports.industry_stats.click_rate,reports.industry_stats.bounce_rate,reports.industry_stats.unopen_rate,reports.industry_stats.unsub_rate,reports.industry_stats.abuse_rate,reports.list_stats.sub_rate,reports.list_stats.unsub_rate,reports.list_stats.open_rate,reports.list_stats.proxy_excluded_open_rate,reports.list_stats.click_rate,reports.ecommerce.total_orders,reports.ecommerce.total_spent,reports.ecommerce.total_revenue,reports.ecommerce.currency_code,reports.delivery_status.enabled,reports.delivery_status.can_cancel,reports.delivery_status.status,reports.delivery_status.emails_sent,reports.delivery_status.emails_canceled,total_items"
)

// ListReports returns aggregate campaign reports without member activity.
func (client *Client) ListReports(ctx context.Context, input ListReportsRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "list_reports"
	if !validReportRequest(input) {
		return ReportPage{}, invalidArgument(operation, "pagination, campaign type, or send-time range is invalid")
	}
	query := make(url.Values)
	setPagination(query, input.Page)
	setStringQuery(query, "type", string(input.Type))
	setStringQuery(query, "since_send_time", input.SinceSendTime)
	setStringQuery(query, "before_send_time", input.BeforeSendTime)
	query.Set("fields", reportCollectionFields)
	var page ReportPage
	meta, _, err := client.getJSON(ctx, operation, "/reports", query, &page, options...)
	if err != nil {
		return ReportPage{}, err
	}
	page.Page, page.Meta = input.Page, meta
	if !validReportPage(page) {
		return ReportPage{}, platformContractError(operation, "Mailchimp returned an invalid report page or total_items value")
	}
	return page, nil
}

// GetReport returns aggregate report details for one sent campaign.
func (client *Client) GetReport(ctx context.Context, input GetReportRequest, options ...socialhub.CallOption) (CampaignReport, error) {
	const operation = "get_report"
	if !validResourceID(input.CampaignID) {
		return CampaignReport{}, invalidArgument(operation, "campaign ID must be a safe bounded path segment")
	}
	query := url.Values{"fields": {reportFields}}
	var report CampaignReport
	meta, _, err := client.getJSON(ctx, operation, "/reports/"+input.CampaignID, query, &report, options...)
	if err != nil {
		return CampaignReport{}, err
	}
	report.Meta = meta
	if !validReport(report, input.CampaignID) {
		return CampaignReport{}, platformContractError(operation, "Mailchimp returned an absent, mismatched, or invalid campaign report")
	}
	return report, nil
}

var _ ReadWorkflow = (*Client)(nil)

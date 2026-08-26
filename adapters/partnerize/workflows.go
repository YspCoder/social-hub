package partnerize

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetPartner(
	ctx context.Context,
	options ...socialhub.CallOption,
) (PartnerResponse, error) {
	const operation = "get_partner"
	var output PartnerResponse
	metadata, err := client.getJSON(ctx, operation, client.partnerPath(""), nil, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, err
	}
	if err := validatePartnerResponse(operation, output, client.publisherID); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) ListCampaigns(
	ctx context.Context,
	input ListCampaignsRequest,
	options ...socialhub.CallOption,
) (CampaignsResponse, error) {
	const operation = "list_campaigns"
	if !validListCampaigns(input) {
		return CampaignsResponse{}, invalidArgument(operation, "campaign status must be approved, pending, or rejected")
	}
	path := client.partnerPath("/campaign/" + string(input.Status))
	var output CampaignsResponse
	metadata, err := client.getJSON(ctx, operation, path, nil, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, err
	}
	if err := validateCampaignsResponse(operation, output); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) ListCreatives(
	ctx context.Context,
	input ListCreativesRequest,
	options ...socialhub.CallOption,
) (CreativesResponse, error) {
	const operation = "list_creatives"
	if !validListCreatives(input) {
		return CreativesResponse{}, invalidArgument(operation, "campaign ID, active flag, tags, or creative type IDs are invalid")
	}
	query := make(url.Values)
	if input.Active != "" {
		query.Set("status", string(input.Active))
	}
	if input.Tags != "" {
		query.Set("tags", input.Tags)
	}
	for _, identifier := range input.CreativeTypeIDs {
		query.Add("creative_type_id", identifier)
	}
	path := client.partnerPath("/campaign/" + input.CampaignID + "/creative")
	var output CreativesResponse
	metadata, err := client.getJSON(ctx, operation, path, query, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, err
	}
	if err := validateCreativesResponse(operation, output, input.CampaignID); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) CreateTrackingLink(
	ctx context.Context,
	input CreateTrackingLinkRequest,
	options ...socialhub.CallOption,
) (TrackingLinkResponse, error) {
	const operation = "create_tracking_link"
	if !validCreateTrackingLink(input) {
		return TrackingLinkResponse{}, invalidArgument(operation, "campaign, destination URL, description, or tracking parameters are invalid")
	}
	payload := struct {
		CampaignID     string         `json:"campaign_id"`
		Description    string         `json:"description,omitempty"`
		DestinationURL string         `json:"destination_url,omitempty"`
		Params         []KeyValuePair `json:"params,omitempty"`
		Active         *bool          `json:"active,omitempty"`
	}{
		CampaignID: input.CampaignID, Description: input.Description, DestinationURL: input.DestinationURL,
		Params: append([]KeyValuePair(nil), input.Params...), Active: input.Active,
	}
	var output TrackingLinkResponse
	metadata, err := client.postJSON(ctx, operation, client.trackingLinksPath(), nil, payload, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, withMutationOutcome(operation, metadata.RequestID, err)
	}
	if err := validateTrackingLinkResponse(operation, output, input.CampaignID); err != nil {
		return output, withMutationOutcome(operation, metadata.RequestID, withHTTPStatus(err, http.StatusOK))
	}
	return output, nil
}

func (client *Client) ListConversions(
	ctx context.Context,
	input ListConversionsRequest,
	options ...socialhub.CallOption,
) (ConversionsResponse, error) {
	const operation = "list_conversions"
	if !validListConversions(input) {
		return ConversionsResponse{}, invalidArgument(operation, "date range, currency, pivot, statuses, or pagination is invalid")
	}
	query := url.Values{"start_date": {input.StartDate.Format(time.RFC3339)}}
	setOptionalTime(query, "end_date", input.EndDate)
	setOptionalQuery(query, "text_date", input.TextDate)
	setOptionalQuery(query, "timezone", input.Timezone)
	setOptionalQuery(query, "currency[]", string(input.Currency))
	setOptionalQuery(query, "date_type", input.DateType)
	if input.Pivot != "" {
		key := "multipivot[" + string(input.Pivot) + "][]"
		for _, value := range input.PivotValues {
			query.Add(key, value)
		}
	}
	for _, status := range input.Statuses {
		query.Add("statuses[]", string(status))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.CursorID > 0 {
		query.Set("cursor_id", strconv.FormatInt(input.CursorID, 10))
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	setOptionalTime(query, "invoice_created_date", input.InvoiceCreatedDate)
	if input.IncludePaymentInfo {
		query.Set("include_payment_info", "true")
	}
	var output ConversionsResponse
	metadata, err := client.getJSON(ctx, operation, client.conversionsPath(), query, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, err
	}
	if err := validateConversionsResponse(operation, output, client.publisherID); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) partnerPath(suffix string) string {
	return "/user/publisher/" + client.publisherID + suffix
}

func (client *Client) trackingLinksPath() string {
	return "/v2/publishers/" + client.publisherID + "/links"
}

func (client *Client) conversionsPath() string {
	return "/reporting/report_publisher/publisher/" + client.publisherID + "/conversion.json"
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalTime(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, value.Format(time.RFC3339))
	}
}

var _ PartnerWorkflow = (*Client)(nil)

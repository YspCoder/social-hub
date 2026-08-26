package impactpartner

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListPrograms(
	ctx context.Context,
	input ListProgramsRequest,
	options ...socialhub.CallOption,
) (ProgramsResponse, error) {
	const operation = "list_programs"
	if !validListPrograms(input) {
		return ProgramsResponse{}, invalidArgument(operation, "insertion order status is invalid")
	}
	query := make(url.Values)
	if input.InsertionOrderStatus != "" {
		query.Set("InsertionOrderStatus", string(input.InsertionOrderStatus))
	}
	var output ProgramsResponse
	metadata, err := client.getJSON(ctx, operation, client.partnerPath("/Campaigns"), query, &output, options...)
	if err != nil {
		return ProgramsResponse{}, err
	}
	output.Meta = metadata
	if err := validateProgramsResponse(operation, output); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) SearchCatalogItems(
	ctx context.Context,
	input SearchCatalogItemsRequest,
	options ...socialhub.CallOption,
) (CatalogItemsResponse, error) {
	const operation = "search_catalog_items"
	if !validSearchCatalogItems(input) {
		return CatalogItemsResponse{}, invalidArgument(operation, "keyword or pagination is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "Keyword", input.Keyword)
	if input.PageSize > 0 {
		query.Set("PageSize", strconv.Itoa(input.PageSize))
	}
	if input.Page > 0 {
		query.Set("Page", strconv.Itoa(input.Page))
	}
	var output CatalogItemsResponse
	metadata, err := client.getJSON(ctx, operation, client.partnerPath("/Catalogs/ItemSearch"), query, &output, options...)
	if err != nil {
		return CatalogItemsResponse{}, err
	}
	output.Meta = metadata
	if err := validateCatalogItemsResponse(operation, output); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) GetCatalogItem(
	ctx context.Context,
	input GetCatalogItemRequest,
	options ...socialhub.CallOption,
) (CatalogItem, error) {
	const operation = "get_catalog_item"
	if !validGetCatalogItem(input) {
		return CatalogItem{}, invalidArgument(operation, "catalog ID or item ID is invalid")
	}
	path := client.partnerPath("/Catalogs/" + input.CatalogID + "/Items/" + input.ItemID)
	var output CatalogItem
	metadata, err := client.getJSON(ctx, operation, path, nil, &output, options...)
	if err != nil {
		return CatalogItem{}, err
	}
	output.Meta = metadata
	if err := validateCatalogItemResponse(operation, output, input.CatalogID, input.ItemID); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) CreateTrackingLink(
	ctx context.Context,
	input CreateTrackingLinkRequest,
	options ...socialhub.CallOption,
) (TrackingLink, error) {
	const operation = "create_tracking_link"
	if !validCreateTrackingLink(input) {
		return TrackingLink{}, invalidArgument(operation, "program ID, link type, destination, or attribution parameter is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "Type", string(input.Type))
	setOptionalQuery(query, "CustomPath", input.CustomPath)
	setOptionalQuery(query, "AdId", input.AdID)
	setOptionalQuery(query, "DeepLink", input.DeepLink)
	setOptionalQuery(query, "MediaPartnerPropertyId", input.MediaPartnerPropertyID)
	setOptionalQuery(query, "subId1", input.SubID1)
	setOptionalQuery(query, "subId2", input.SubID2)
	setOptionalQuery(query, "subId3", input.SubID3)
	setOptionalQuery(query, "sharedId", input.SharedID)
	path := client.partnerPath("/Programs/" + input.ProgramID + "/TrackingLinks")
	var output TrackingLink
	metadata, err := client.postWithoutBody(ctx, operation, path, query, &output, options...)
	output.Meta = metadata
	if err != nil {
		return output, withMutationOutcome(operation, metadata.RequestID, err)
	}
	if err := validateTrackingLinkResponse(operation, output); err != nil {
		return output, withMutationOutcome(operation, metadata.RequestID, withHTTPStatus(err, http.StatusOK))
	}
	return output, nil
}

func (client *Client) ListActions(
	ctx context.Context,
	input ListActionsRequest,
	options ...socialhub.CallOption,
) (ActionsResponse, error) {
	const operation = "list_actions"
	if !validListActions(input, client.clock.Now()) {
		return ActionsResponse{}, invalidArgument(operation, "campaign, state, date window, or pagination is invalid")
	}
	query := make(url.Values)
	if input.CampaignID > 0 {
		query.Set("CampaignId", strconv.FormatInt(input.CampaignID, 10))
	}
	setOptionalQuery(query, "State", string(input.State))
	setOptionalTime(query, "ActionDateStart", input.ActionDateStart)
	setOptionalTime(query, "ActionDateEnd", input.ActionDateEnd)
	setOptionalTime(query, "StartDate", input.StartDate)
	setOptionalTime(query, "EndDate", input.EndDate)
	setOptionalTime(query, "LockingDateStart", input.LockingDateStart)
	setOptionalTime(query, "LockingDateEnd", input.LockingDateEnd)
	if input.Page > 0 {
		query.Set("Page", strconv.Itoa(input.Page))
	}
	if input.PageSize > 0 {
		query.Set("PageSize", strconv.Itoa(input.PageSize))
	}
	var output ActionsResponse
	metadata, err := client.getJSON(ctx, operation, client.partnerPath("/Actions"), query, &output, options...)
	if err != nil {
		return ActionsResponse{}, err
	}
	output.Meta = metadata
	if err := validateActionsResponse(operation, output); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) partnerPath(suffix string) string {
	return "/Mediapartners/" + client.accountSID + suffix
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

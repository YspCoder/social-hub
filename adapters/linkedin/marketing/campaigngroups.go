package marketing

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

var allCampaignGroupStatuses = []Status{
	StatusActive, StatusPaused, StatusDraft, StatusArchived, StatusCanceled, StatusPendingDeletion, StatusRemoved,
}

type campaignGroupPage struct {
	Elements []CampaignGroup `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

type createCampaignGroupPayload struct {
	Account     string      `json:"account"`
	Name        string      `json:"name"`
	RunSchedule RunSchedule `json:"runSchedule"`
	Status      Status      `json:"status"`
	TotalBudget *Money      `json:"totalBudget,omitempty"`
}

func (client *Client) ListCampaignGroups(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[CampaignGroup], error) {
	const operation = "campaign_groups_list"
	if !validPage(input.Cursor, input.MaxResults, 1000) || !validStatuses(input.Statuses, validGroupStatus) {
		return socialhub.Page[CampaignGroup]{}, invalidArgument(operation, "statuses, page token, or page size is invalid")
	}
	statuses := input.Statuses
	if len(statuses) == 0 {
		statuses = allCampaignGroupStatuses
	}
	var response campaignGroupPage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("adCampaignGroups"), searchQuery(statuses, input.Cursor, input.MaxResults), "", &response, options...); err != nil {
		return socialhub.Page[CampaignGroup]{}, err
	}
	for index := range response.Elements {
		if err := client.validateCampaignGroup(operation, &response.Elements[index], ""); err != nil {
			return socialhub.Page[CampaignGroup]{}, err
		}
	}
	return cursorPage(response.Elements, response.Metadata.NextPageToken), nil
}

func (client *Client) GetCampaignGroup(ctx context.Context, id string, options ...socialhub.CallOption) (*CampaignGroup, error) {
	const operation = "campaign_group_get"
	if !validNumericID(id) {
		return nil, invalidArgument(operation, "Campaign Group ID must be numeric")
	}
	var response CampaignGroup
	if _, err := client.getJSON(ctx, operation, client.resourcePath("adCampaignGroups")+"/"+id, "", "", &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCampaignGroup(operation, &response, id); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateCampaignGroup(ctx context.Context, input CreateCampaignGroupRequest, options ...socialhub.CallOption) (*CampaignGroup, error) {
	const operation = "campaign_group_create"
	if !validText(input.Name, 200) || !validSchedule(input.RunSchedule) || input.TotalBudget != nil && !validMoney(*input.TotalBudget) {
		return nil, invalidArgument(operation, "name, run schedule, or total budget is invalid")
	}
	payload := createCampaignGroupPayload{
		Account: client.accountURN(), Name: input.Name, RunSchedule: input.RunSchedule,
		Status: StatusDraft, TotalBudget: input.TotalBudget,
	}
	metadata, err := client.writeJSON(ctx, operation, client.resourcePath("adCampaignGroups"), "", payload, nil, options...)
	if err != nil {
		return nil, err
	}
	id, err := numericIDFromHeader(operation, metadata, campaignGroupURNPrefix)
	if err != nil {
		return nil, err
	}
	return client.GetCampaignGroup(ctx, id, options...)
}

func (client *Client) UpdateCampaignGroup(ctx context.Context, id string, input UpdateCampaignGroupRequest, options ...socialhub.CallOption) (*CampaignGroup, error) {
	const operation = "campaign_group_update"
	if !validNumericID(id) || input.Name != nil && !validText(*input.Name, 200) || input.TotalBudget != nil && !validMoney(*input.TotalBudget) ||
		input.Name == nil && input.TotalBudget == nil {
		return nil, invalidArgument(operation, "Campaign Group ID or update fields are invalid")
	}
	set := map[string]any{}
	if input.Name != nil {
		set["name"] = *input.Name
	}
	if input.TotalBudget != nil {
		set["totalBudget"] = input.TotalBudget
	}
	return client.updateCampaignGroup(ctx, operation, id, set, options...)
}

func (client *Client) SetCampaignGroupStatus(ctx context.Context, id string, status Status, options ...socialhub.CallOption) (*CampaignGroup, error) {
	const operation = "campaign_group_status"
	if !validNumericID(id) || !validMutationStatus(status) {
		return nil, invalidArgument(operation, "Campaign Group ID or status is invalid")
	}
	return client.updateCampaignGroup(ctx, operation, id, map[string]any{"status": status}, options...)
}

func (client *Client) ArchiveCampaignGroup(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "campaign_group_archive"
	if !validNumericID(id) {
		return invalidArgument(operation, "Campaign Group ID must be numeric")
	}
	_, err := client.updateCampaignGroup(ctx, operation, id, map[string]any{"status": StatusArchived}, options...)
	return err
}

func (client *Client) updateCampaignGroup(ctx context.Context, operation, id string, set map[string]any, options ...socialhub.CallOption) (*CampaignGroup, error) {
	payload := map[string]any{"patch": map[string]any{"$set": set}}
	if _, err := client.writeJSON(ctx, operation, client.resourcePath("adCampaignGroups")+"/"+id, "PARTIAL_UPDATE", payload, nil, options...); err != nil {
		return nil, err
	}
	return client.GetCampaignGroup(ctx, id, options...)
}

func (client *Client) validateCampaignGroup(operation string, value *CampaignGroup, expectedID string) error {
	if !validNumericID(string(value.ID)) || expectedID != "" && string(value.ID) != expectedID {
		return platformContractError(operation, "LinkedIn returned a missing or mismatched Campaign Group ID")
	}
	if value.Account != client.accountURN() {
		return platformContractError(operation, "LinkedIn returned a Campaign Group owned by another Ad Account")
	}
	return nil
}

func searchQuery(statuses []Status, cursor string, maxResults int) string {
	values := make([]string, len(statuses))
	for index, status := range statuses {
		values[index] = string(status)
	}
	parts := []string{"q=search", "search=(status:(values:List(" + strings.Join(values, ",") + ")))"}
	if maxResults > 0 {
		parts = append(parts, "pageSize="+strconv.Itoa(maxResults))
	}
	if cursor != "" {
		parts = append(parts, "pageToken="+url.QueryEscape(cursor))
	}
	return strings.Join(parts, "&")
}

func numericIDFromHeader(operation string, metadata transport.ResponseMetadata, prefix string) (string, error) {
	id := metadata.Header.Get("X-RestLi-Id")
	if decoded, err := url.QueryUnescape(id); err == nil {
		id = decoded
	}
	id = strings.TrimPrefix(id, prefix)
	if !validNumericID(id) {
		return "", platformContractError(operation, "LinkedIn omitted a valid X-RestLi-Id response header")
	}
	return id, nil
}

func cursorPage[T any](elements []T, cursor string) socialhub.Page[T] {
	var next *string
	if cursor != "" {
		value := cursor
		next = &value
	}
	return socialhub.Page[T]{Items: elements, NextCursor: next, HasMore: cursor != ""}
}

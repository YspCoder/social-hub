package marketing

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

type creativePage struct {
	Elements []Creative `json:"elements"`
	Metadata struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"metadata"`
}

type creativeBatchResponse struct {
	Elements []struct {
		Status int            `json:"status"`
		ID     string         `json:"id"`
		Error  *errorResponse `json:"error,omitempty"`
	} `json:"elements"`
}

func (client *Client) ListCreatives(ctx context.Context, input ListCreativesRequest, options ...socialhub.CallOption) (socialhub.Page[Creative], error) {
	const operation = "creatives_list"
	if !validPage(input.Cursor, input.MaxResults, 100) {
		return socialhub.Page[Creative]{}, invalidArgument(operation, "page token or page size is invalid")
	}
	parts := []string{"q=criteria", "sortOrder=ASCENDING"}
	if input.MaxResults > 0 {
		parts = append(parts, "pageSize="+strconv.Itoa(input.MaxResults))
	}
	if input.Cursor != "" {
		parts = append(parts, "pageToken="+url.QueryEscape(input.Cursor))
	}
	var response creativePage
	if _, err := client.getJSON(ctx, operation, client.resourcePath("creatives"), strings.Join(parts, "&"), "FINDER", &response, options...); err != nil {
		return socialhub.Page[Creative]{}, err
	}
	for index := range response.Elements {
		if err := client.validateCreative(operation, &response.Elements[index], "", ""); err != nil {
			return socialhub.Page[Creative]{}, err
		}
	}
	return cursorPage(response.Elements, response.Metadata.NextPageToken), nil
}

func (client *Client) GetCreative(ctx context.Context, id string, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_get"
	if !validNumericURN(id, creativeURNPrefix) {
		return nil, invalidArgument(operation, "Creative ID must be a sponsoredCreative URN")
	}
	var response Creative
	if _, err := client.getJSON(ctx, operation, client.resourcePath("creatives")+"/"+id, "", "", &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateCreative(operation, &response, id, ""); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) CreateCreative(ctx context.Context, input CreateCreativeRequest, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_create"
	if !validNumericID(input.CampaignID) || !validContentURN(input.ContentURN) || !validOptionalText(input.Name, 200) {
		return nil, invalidArgument(operation, "Campaign ID, share/ugcPost content URN, or name is invalid")
	}
	element := struct {
		Campaign       string          `json:"campaign"`
		Content        CreativeContent `json:"content"`
		Name           string          `json:"name,omitempty"`
		IntendedStatus Status          `json:"intendedStatus"`
	}{
		Campaign: campaignURNPrefix + input.CampaignID, Content: CreativeContent{Reference: input.ContentURN},
		Name: input.Name, IntendedStatus: StatusDraft,
	}
	var response creativeBatchResponse
	if _, err := client.writeJSON(ctx, operation, client.resourcePath("creatives"), "BATCH_CREATE", map[string]any{"elements": []any{element}}, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Elements) != 1 || response.Elements[0].Status < 200 || response.Elements[0].Status >= 300 ||
		!validNumericURN(response.Elements[0].ID, creativeURNPrefix) {
		return nil, creativeBatchError(operation, response)
	}
	creative, err := client.GetCreative(ctx, response.Elements[0].ID, options...)
	if err != nil {
		return nil, err
	}
	if creative.Campaign != campaignURNPrefix+input.CampaignID {
		return nil, platformContractError(operation, "LinkedIn returned a Creative in another Campaign")
	}
	return creative, nil
}

func (client *Client) SetCreativeStatus(ctx context.Context, id string, status Status, options ...socialhub.CallOption) (*Creative, error) {
	const operation = "creative_status"
	if !validNumericURN(id, creativeURNPrefix) || !validMutationStatus(status) {
		return nil, invalidArgument(operation, "Creative ID or intended status is invalid")
	}
	return client.updateCreative(ctx, operation, id, status, options...)
}

func (client *Client) ArchiveCreative(ctx context.Context, id string, options ...socialhub.CallOption) error {
	const operation = "creative_archive"
	if !validNumericURN(id, creativeURNPrefix) {
		return invalidArgument(operation, "Creative ID must be a sponsoredCreative URN")
	}
	_, err := client.updateCreative(ctx, operation, id, StatusArchived, options...)
	return err
}

func (client *Client) updateCreative(ctx context.Context, operation, id string, status Status, options ...socialhub.CallOption) (*Creative, error) {
	payload := map[string]any{"patch": map[string]any{"$set": map[string]any{"intendedStatus": status}}}
	if _, err := client.writeJSON(ctx, operation, client.resourcePath("creatives")+"/"+id, "PARTIAL_UPDATE", payload, nil, options...); err != nil {
		return nil, err
	}
	return client.GetCreative(ctx, id, options...)
}

func (client *Client) validateCreative(operation string, value *Creative, expectedID, expectedCampaignID string) error {
	if !validNumericURN(value.ID, creativeURNPrefix) || expectedID != "" && value.ID != expectedID {
		return platformContractError(operation, "LinkedIn returned a missing or mismatched Creative ID")
	}
	if value.Account != "" && value.Account != client.accountURN() {
		return platformContractError(operation, "LinkedIn returned a Creative owned by another Ad Account")
	}
	if !validNumericURN(value.Campaign, campaignURNPrefix) || expectedCampaignID != "" && value.Campaign != campaignURNPrefix+expectedCampaignID {
		return platformContractError(operation, "LinkedIn returned an invalid or mismatched Creative Campaign")
	}
	return nil
}

func creativeBatchError(operation string, response creativeBatchResponse) error {
	if len(response.Elements) == 1 && response.Elements[0].Error != nil {
		item := response.Elements[0]
		message := firstNonEmpty(item.Error.Message, item.Error.ErrorDescription)
		code, class := classifyError(item.Status, message)
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
			HTTPStatus: item.Status, PlatformCode: firstNonEmpty(item.Error.Code, item.Error.Error),
			PlatformMessage: boundedMessage(redactSensitive(message), 512),
		}
	}
	return platformContractError(operation, "LinkedIn did not return exactly one successful Creative batch result")
}

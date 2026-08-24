package xiaohongshumarketing

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

const (
	campaignStatusPath = "/api/open/jg/campaign/status/update"
	unitStatusPath     = "/api/open/jg/unit/update/status"
	// The missing slash between creativity and status is the provider's
	// documented route, not a normalization typo.
	creativeStatusPath = "/api/open/jg/creativitystatus/update"
)

func (client *Client) ResumeCampaigns(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "campaign_resume", campaignStatusPath, "campaign_ids", "action_type", ids, StatusActionResume, options...)
}

func (client *Client) PauseCampaigns(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "campaign_pause", campaignStatusPath, "campaign_ids", "action_type", ids, StatusActionPause, options...)
}

func (client *Client) DeleteCampaigns(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "campaign_delete", campaignStatusPath, "campaign_ids", "action_type", ids, StatusActionDelete, options...)
}

func (client *Client) ResumeUnits(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "unit_resume", unitStatusPath, "unit_ids", "status", ids, StatusActionResume, options...)
}

func (client *Client) PauseUnits(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "unit_pause", unitStatusPath, "unit_ids", "status", ids, StatusActionPause, options...)
}

func (client *Client) DeleteUnits(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "unit_delete", unitStatusPath, "unit_ids", "status", ids, StatusActionDelete, options...)
}

func (client *Client) ResumeCreatives(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "creative_resume", creativeStatusPath, "creativity_ids", "action_type", ids, StatusActionResume, options...)
}

func (client *Client) PauseCreatives(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "creative_pause", creativeStatusPath, "creativity_ids", "action_type", ids, StatusActionPause, options...)
}

func (client *Client) DeleteCreatives(ctx context.Context, ids []uint64, options ...socialhub.CallOption) (MutationResult, error) {
	return client.updateStatus(ctx, "creative_delete", creativeStatusPath, "creativity_ids", "action_type", ids, StatusActionDelete, options...)
}

func (client *Client) updateStatus(
	ctx context.Context,
	operation string,
	path string,
	idField string,
	actionField string,
	ids []uint64,
	action StatusAction,
	options ...socialhub.CallOption,
) (MutationResult, error) {
	result := MutationResult{RequestedIDs: append([]uint64(nil), ids...)}
	if !validIDs(ids, 20, true) || !validStatusAction(action) {
		return result, invalidArgument(operation, "1..20 unique positive IDs and a valid status action are required")
	}
	body := map[string]any{
		"advertiser_id": client.advertiserID,
		idField:         append([]uint64(nil), ids...),
		actionField:     action,
	}
	raw, requestID, err := client.doJSON(ctx, operation, path, body, true, options...)
	result.RequestID = requestID
	if err != nil {
		return result, err
	}
	var data map[string]json.RawMessage
	if err := decodeRequiredData(operation, raw, &data); err != nil {
		return result, outcomeUnknownError(operation, err)
	}
	encodedIDs, found := data[idField]
	if !found || json.Unmarshal(encodedIDs, &result.AcknowledgedIDs) != nil {
		return result, outcomeUnknownError(operation, platformContractError(operation, "Spotlight status response omitted acknowledged IDs"))
	}
	if !validIDs(result.AcknowledgedIDs, 20, false) || !subsetOf(result.AcknowledgedIDs, ids) {
		return result, platformContractError(operation, "Spotlight status response contained invalid or unexpected IDs")
	}
	if len(result.AcknowledgedIDs) != len(ids) {
		return result, partialMutationError(operation, requestID)
	}
	return result, nil
}

func subsetOf(values, allowed []uint64) bool {
	set := make(map[uint64]struct{}, len(allowed))
	for _, value := range allowed {
		set[value] = struct{}{}
	}
	for _, value := range values {
		if _, found := set[value]; !found {
			return false
		}
	}
	return true
}

package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

type batchErrorWire struct {
	ID         int64  `json:"id,omitempty"`
	CampaignID int64  `json:"campaign_id,omitempty"`
	UnitID     int64  `json:"unit_id,omitempty"`
	CreativeID int64  `json:"creative_id,omitempty"`
	Code       int64  `json:"error_code,omitempty"`
	Message    string `json:"error_msg,omitempty"`
}

func (value batchErrorWire) resourceID() int64 {
	for _, id := range []int64{value.ID, value.CampaignID, value.UnitID, value.CreativeID} {
		if id != 0 {
			return id
		}
	}
	return 0
}

type statusData struct {
	CampaignID  int64            `json:"campaign_id,omitempty"`
	CampaignIDs []int64          `json:"campaign_ids,omitempty"`
	UnitID      int64            `json:"unit_id,omitempty"`
	UnitIDs     []int64          `json:"unit_ids,omitempty"`
	CreativeID  int64            `json:"creative_id,omitempty"`
	CreativeIDs []int64          `json:"creative_ids,omitempty"`
	Errors      []batchErrorWire `json:"errors,omitempty"`
}

func (client *Client) setStatus(ctx context.Context, operation, path, idField string, resourceID int64, status PutStatus, options ...socialhub.CallOption) (BatchResult, error) {
	if !validID(resourceID) || !validPutStatus(status, true) {
		return BatchResult{}, invalidArgument(operation, "a resource ID and delivering, paused, or deleted status are required")
	}
	body := map[string]any{"advertiser_id": client.advertiserID, idField: resourceID, "put_status": status}
	var response apiEnvelope[statusData]
	header, err := client.postJSON(ctx, operation, path, body, &response, options...)
	if err != nil {
		return BatchResult{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return BatchResult{}, err
	}
	succeeded := make([]int64, 0, 1)
	for _, id := range []int64{data.CampaignID, data.UnitID, data.CreativeID} {
		if id != 0 {
			succeeded = append(succeeded, id)
		}
	}
	succeeded = append(succeeded, data.CampaignIDs...)
	succeeded = append(succeeded, data.UnitIDs...)
	succeeded = append(succeeded, data.CreativeIDs...)
	errors := make([]BatchError, 0, len(data.Errors))
	for _, item := range data.Errors {
		id := item.resourceID()
		if id == resourceID {
			return BatchResult{}, platformContractError(operation, "Kuaishou reported a per-resource status failure")
		}
		errors = append(errors, BatchError{ID: id, Code: item.Code, Message: boundedMessage(redactSensitive(item.Message), 512)})
	}
	if len(succeeded) > 0 && !containsID(succeeded, resourceID) {
		return BatchResult{}, platformContractError(operation, "Kuaishou status response omitted the requested resource")
	}
	return BatchResult{SucceededIDs: succeeded, Errors: errors}, nil
}

package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

type statusErrorWire struct {
	CampaignID string `json:"campaign_id,omitempty"`
	AdGroupID  string `json:"adgroup_id,omitempty"`
	AdID       string `json:"ad_id,omitempty"`
	Message    string `json:"error_message,omitempty"`
}

func (value statusErrorWire) resourceID() string {
	return firstNonEmpty(value.CampaignID, value.AdGroupID, value.AdID)
}

type statusData struct {
	CampaignIDs []string          `json:"campaign_ids,omitempty"`
	AdGroupIDs  []string          `json:"adgroup_ids,omitempty"`
	AdIDs       []string          `json:"ad_ids,omitempty"`
	Status      OperationStatus   `json:"status,omitempty"`
	ErrorList   []statusErrorWire `json:"error_list,omitempty"`
}

func (client *Client) setStatus(ctx context.Context, operation, path, idField, resourceID string, status OperationStatus, options ...socialhub.CallOption) (BatchResult, error) {
	if !validID(resourceID) || !validOperationStatus(status) {
		return BatchResult{}, invalidArgument(operation, "a numeric resource ID and ENABLE, DISABLE, or DELETE status are required")
	}
	body := map[string]any{"advertiser_id": client.advertiserID, idField: []string{resourceID}, "operation_status": status}
	var response apiEnvelope[statusData]
	header, err := client.postJSON(ctx, operation, path, body, &response, options...)
	if err != nil {
		return BatchResult{}, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return BatchResult{}, err
	}
	if data.Status != "" && data.Status != status {
		return BatchResult{}, platformContractError(operation, "TikTok returned a mismatched operation status")
	}
	succeeded := append([]string(nil), data.CampaignIDs...)
	succeeded = append(succeeded, data.AdGroupIDs...)
	succeeded = append(succeeded, data.AdIDs...)
	errors := make([]BatchError, 0, len(data.ErrorList))
	failedRequested := false
	for _, item := range data.ErrorList {
		id := item.resourceID()
		if id != "" && !validID(id) {
			return BatchResult{}, platformContractError(operation, "TikTok returned an invalid failed resource ID")
		}
		failedRequested = failedRequested || id == resourceID
		errors = append(errors, BatchError{ID: id, Message: boundedMessage(redactSensitive(item.Message), 512)})
	}
	result := BatchResult{SucceededIDs: succeeded, Errors: errors}
	if failedRequested {
		return result, platformContractError(operation, "TikTok reported that the requested status mutation failed")
	}
	if !containsID(succeeded, resourceID) {
		return result, platformContractError(operation, "TikTok status response omitted the requested resource")
	}
	return result, nil
}

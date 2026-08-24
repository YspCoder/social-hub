package xiaohongshureporting

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

const balancePath = "/api/open/jg/account/budget/info"

func (client *Client) Balance(ctx context.Context, options ...socialhub.CallOption) (AccountBalance, error) {
	const operation = "balance"
	raw, requestID, err := client.doJSON(ctx, operation, balancePath, struct {
		AdvertiserID uint64 `json:"advertiser_id"`
	}{AdvertiserID: client.advertiserID}, options...)
	if err != nil {
		return AccountBalance{}, err
	}
	data, found := rawObjectField(raw, "data")
	if !found || isJSONNull(data) {
		return AccountBalance{}, platformContractError(operation, "Spotlight balance response omitted data")
	}
	var balance AccountBalance
	if err := json.Unmarshal(data, &balance); err != nil || !validBalance(balance) {
		return AccountBalance{}, platformContractError(operation, "Spotlight returned invalid balance data")
	}
	balance.RequestID = requestID
	return balance, nil
}

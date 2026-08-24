package ads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "ad_account_get"
	var response singleResponse[AdAccount]
	if _, err := client.getJSON(ctx, operation, "/ad_accounts/"+client.adAccountID, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Data.ID != client.adAccountID {
		return nil, platformContractError(operation, "Reddit returned another Ad Account")
	}
	return &response.Data, nil
}

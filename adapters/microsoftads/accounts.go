package microsoftads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	const operation = "get_account"
	var response struct {
		Account Account `json:"Account"`
	}
	_, err := client.postJSON(ctx, operation, client.customer, "/Account/Query", struct {
		AccountID string `json:"AccountId"`
	}{AccountID: client.customerAccountID}, &response, options...)
	if err != nil {
		return nil, err
	}
	if response.Account.ID == "" {
		return nil, platformContractError(operation, "response did not contain an account")
	}
	if response.Account.ID != client.customerAccountID {
		return nil, platformContractError(operation, "response account does not match configured customer_account_id")
	}
	return &response.Account, nil
}

func (client *Client) validateAccount(ctx context.Context, options ...socialhub.CallOption) error {
	_, err := client.GetAccount(ctx, options...)
	return err
}

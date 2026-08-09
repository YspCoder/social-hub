package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "ad_account_get"
	var response AdAccount
	if _, err := client.getJSON(ctx, operation, client.accountPath(), "", "", &response, options...); err != nil {
		return nil, err
	}
	if string(response.ID) != client.adAccountID {
		return nil, platformContractError(operation, "LinkedIn returned a missing or mismatched Ad Account ID")
	}
	return &response, nil
}

func (client *Client) accountPath() string { return "/adAccounts/" + client.adAccountID }

func (client *Client) accountURN() string { return accountURNPrefix + client.adAccountID }

func (client *Client) resourcePath(resource string) string {
	return client.accountPath() + "/" + resource
}

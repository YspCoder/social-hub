package kakaomoment

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "account_get"
	var account AdAccount
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"adAccounts/"+formatID(client.adAccountID), nil, nil, &account, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if account.ID != client.adAccountID || !validText(account.Name, 1024) || !validConfig(account.Config, true) ||
		account.BizWalletID <= 0 {
		return nil, platformContractError(operation, "Kakao Moment returned an invalid or different ad account")
	}
	return &account, nil
}

func (client *Client) GetBalance(ctx context.Context, options ...socialhub.CallOption) (*Balance, error) {
	const operation = "account_balance"
	var balance Balance
	_, err := client.doJSON(
		ctx, operation, []string{ScopeManagement}, http.MethodGet,
		"adAccounts/balance", nil, nil, &balance, false, options...,
	)
	if err != nil {
		return nil, err
	}
	if balance.ID != client.adAccountID || balance.BizWalletID <= 0 || balance.Cash < 0 || balance.FreeCash < 0 {
		return nil, platformContractError(operation, "Kakao Moment returned invalid balance data")
	}
	return &balance, nil
}

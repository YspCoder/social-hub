package taboola

import (
	"context"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) CurrentAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	const operation = "account_current"
	var response Account
	if err := client.getJSON(ctx, operation, "users/current/account", nil, &response, options...); err != nil {
		return nil, err
	}
	if !validPathID(response.AccountID, false) {
		return nil, platformContractError(operation, "response account_id is invalid")
	}
	return &response, nil
}

func (client *Client) AllowedAccounts(ctx context.Context, options ...socialhub.CallOption) ([]Account, error) {
	const operation = "accounts_allowed"
	var response pageEnvelope[Account]
	if err := client.getJSON(ctx, operation, "users/current/allowed-accounts", nil, &response, options...); err != nil {
		return nil, err
	}
	for index := range response.Results {
		if !validPathID(response.Results[index].AccountID, false) {
			return nil, platformContractError(operation, "response account_id is invalid")
		}
	}
	return response.Results, nil
}

func (client *Client) ValidateConfiguredAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	const operation = "account_validate"
	accounts, err := client.AllowedAccounts(ctx, options...)
	if err != nil {
		return nil, err
	}
	for index := range accounts {
		account := &accounts[index]
		if account.AccountID != client.advertiserID {
			continue
		}
		if !containsFold(account.PartnerTypes, "ADVERTISER") || !containsFold(account.CampaignTypes, "PAID") {
			return nil, platformContractError(operation, "configured account is not a paid advertiser account")
		}
		return account, nil
	}
	return nil, platformError(operation, socialhub.CodePermissionDenied, socialhub.ClassUserAction, nil)
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

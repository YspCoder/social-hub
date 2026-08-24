package adsense

import (
	"context"

	"social-hub/pkg/socialhub"
)

type AccountsWorkflow interface {
	GetAccount(context.Context, ...socialhub.CallOption) (*Account, error)
	ListChildAccounts(context.Context, ListRequest, ...socialhub.CallOption) (Page[Account], error)
	GetAdBlockingRecoveryTag(context.Context, ...socialhub.CallOption) (*AdBlockingRecoveryTag, error)
}

func (client *Client) GetAccount(ctx context.Context, options ...socialhub.CallOption) (*Account, error) {
	const operation = "account_get"
	var output Account
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName(), nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != client.accountName() {
		return nil, ownershipError(operation, "account")
	}
	return &output, nil
}

func (client *Client) ListChildAccounts(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Account], error) {
	const operation = "child_accounts_list"
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[Account]{}, err
	}
	var output struct {
		Accounts      []Account `json:"accounts"`
		NextPageToken string    `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+":listChildAccounts", query, &output, options...); err != nil {
		return Page[Account]{}, err
	}
	for _, item := range output.Accounts {
		if !validAccountName(item.Name) || item.Name == client.accountName() {
			return Page[Account]{}, platformContractError(operation, "child account resource name is invalid")
		}
	}
	return Page[Account]{Items: output.Accounts, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetAdBlockingRecoveryTag(ctx context.Context, options ...socialhub.CallOption) (*AdBlockingRecoveryTag, error) {
	const operation = "ad_blocking_recovery_tag_get"
	var output AdBlockingRecoveryTag
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/adBlockingRecoveryTag", nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Tag == "" || len(output.Raw) == 0 {
		return nil, platformContractError(operation, "AdSense returned an invalid recovery tag")
	}
	return &output, nil
}

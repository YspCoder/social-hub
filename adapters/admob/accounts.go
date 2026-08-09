package admob

import (
	"context"

	"social-hub/pkg/socialhub"
)

type AccountsWorkflow interface {
	ListAccounts(context.Context, ListRequest, ...socialhub.CallOption) (Page[PublisherAccount], error)
	GetAccount(context.Context, ...socialhub.CallOption) (*PublisherAccount, error)
}

// ListAccounts returns every publisher account visible to the configured OAuth
// credential. It is the one stable-v1 discovery method not scoped to parent.
func (client *Client) ListAccounts(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[PublisherAccount], error) {
	const operation = "accounts_list"
	if err := client.requireScope(operation, readOnlyScope, reportScope); err != nil {
		return Page[PublisherAccount]{}, err
	}
	query, err := listQuery(operation, input, false)
	if err != nil {
		return Page[PublisherAccount]{}, err
	}
	var output struct {
		Accounts      []PublisherAccount `json:"account"`
		NextPageToken string             `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v1/accounts", query, &output, options...); err != nil {
		return Page[PublisherAccount]{}, err
	}
	if input.PageSize > 0 && len(output.Accounts) > int(input.PageSize) || !validPageToken(output.NextPageToken) {
		return Page[PublisherAccount]{}, platformContractError(operation, "AdMob returned invalid account pagination")
	}
	seen := make(map[string]struct{}, len(output.Accounts))
	for _, account := range output.Accounts {
		if !validPublisherAccount(account, "") {
			return Page[PublisherAccount]{}, platformContractError(operation, "AdMob returned an invalid publisher account")
		}
		if _, exists := seen[account.Name]; exists {
			return Page[PublisherAccount]{}, platformContractError(operation, "AdMob returned duplicate publisher accounts")
		}
		seen[account.Name] = struct{}{}
	}
	return Page[PublisherAccount]{Items: output.Accounts, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetAccount(ctx context.Context, options ...socialhub.CallOption) (*PublisherAccount, error) {
	const operation = "account_get"
	if err := client.requireScope(operation, readOnlyScope, reportScope); err != nil {
		return nil, err
	}
	var output PublisherAccount
	if err := client.getJSON(ctx, operation, "/v1/"+client.accountName(), nil, &output, options...); err != nil {
		return nil, err
	}
	if !validPublisherAccount(output, client.publisherID) {
		return nil, ownershipError(operation, "publisher account")
	}
	return &output, nil
}

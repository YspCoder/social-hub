package ads

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type adAccountPage struct {
	Items    []AdAccount `json:"items"`
	Bookmark string      `json:"bookmark"`
}

func (client *Client) ListAdAccounts(ctx context.Context, input ListAdAccountsRequest, options ...socialhub.CallOption) (socialhub.Page[AdAccount], error) {
	const operation = "ad_accounts_list"
	if !validPage(input.Cursor, input.MaxResults) {
		return socialhub.Page[AdAccount]{}, invalidArgument(operation, "bookmark or page size is invalid")
	}
	query := listQuery(input.Cursor, input.MaxResults)
	if input.IncludeSharedAccounts != nil {
		query.Set("include_shared_accounts", strconv.FormatBool(*input.IncludeSharedAccounts))
	}
	var response adAccountPage
	if _, err := client.getJSON(ctx, operation, "/ad_accounts", query, &response, options...); err != nil {
		return socialhub.Page[AdAccount]{}, err
	}
	for _, account := range response.Items {
		if !validID(account.ID) {
			return socialhub.Page[AdAccount]{}, platformContractError(operation, "Pinterest returned an invalid Ad Account ID")
		}
	}
	return toPage(response.Items, response.Bookmark), nil
}

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "ad_account_get"
	var response AdAccount
	if _, err := client.getJSON(ctx, operation, "/ad_accounts/"+client.adAccountID, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID != client.adAccountID {
		return nil, platformContractError(operation, "Pinterest returned a missing or mismatched Ad Account ID")
	}
	return &response, nil
}

func listQuery(cursor string, maxResults int) url.Values {
	query := url.Values{}
	if cursor != "" {
		query.Set("bookmark", cursor)
	}
	if maxResults > 0 {
		query.Set("page_size", strconv.Itoa(maxResults))
	}
	return query
}

func toPage[T any](items []T, bookmark string) socialhub.Page[T] {
	var next *string
	if bookmark != "" {
		value := bookmark
		next = &value
	}
	return socialhub.Page[T]{Items: items, NextCursor: next, HasMore: bookmark != ""}
}

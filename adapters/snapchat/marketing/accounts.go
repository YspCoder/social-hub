package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

type adAccountItem struct {
	SubRequestStatus      string     `json:"sub_request_status"`
	SubRequestErrorReason string     `json:"sub_request_error_reason,omitempty"`
	Errors                []apiError `json:"errors,omitempty"`
	AdAccount             *AdAccount `json:"adaccount"`
}

type adAccountResponse struct {
	responseMeta
	AdAccounts []adAccountItem `json:"adaccounts"`
}

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "ad_account_get"
	var response adAccountResponse
	if _, err := client.getJSON(ctx, operation, client.accountPath(), nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.AdAccounts) != 1 {
		return nil, platformContractError(operation, "Snapchat did not return exactly one Ad Account")
	}
	item := response.AdAccounts[0]
	if err := checkResponse(operation, response.responseMeta, []subRequestState{{Status: item.SubRequestStatus, Reason: item.SubRequestErrorReason, Errors: item.Errors}}); err != nil {
		return nil, err
	}
	if item.AdAccount == nil || item.AdAccount.ID != client.adAccountID {
		return nil, platformContractError(operation, "Snapchat returned a missing or mismatched Ad Account")
	}
	return item.AdAccount, nil
}

func (client *Client) accountPath() string { return "/adaccounts/" + client.adAccountID }

func (client *Client) accountResourcePath(resource string) string {
	return client.accountPath() + "/" + resource
}

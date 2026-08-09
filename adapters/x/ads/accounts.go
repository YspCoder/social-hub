package ads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdAccount(ctx context.Context, options ...socialhub.CallOption) (*AdAccount, error) {
	const operation = "ad_account_get"
	var response singleResponse[AdAccount]
	if _, err := client.get(ctx, client.accountPath(), nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Data.ID != client.adsAccountID {
		return nil, platformContractError(operation, "X returned a missing or mismatched Ads Account ID")
	}
	return &response.Data, nil
}

func (client *Client) GetAuthenticatedUserAccess(ctx context.Context, options ...socialhub.CallOption) (*AuthenticatedUserAccess, error) {
	const operation = "authenticated_user_access_get"
	var response singleResponse[AuthenticatedUserAccess]
	if _, err := client.get(ctx, client.resourcePath("authenticated_user_access"), nil, &response, options...); err != nil {
		return nil, err
	}
	if !validTweetID(response.Data.UserID) || !validPermissions(response.Data.Permissions) {
		return nil, platformContractError(operation, "X returned invalid authenticated-user access data")
	}
	return &response.Data, nil
}

func (client *Client) getFundingInstrument(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*FundingInstrument, error) {
	var response singleResponse[FundingInstrument]
	if _, err := client.get(ctx, client.resourcePath("funding_instruments")+"/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.Data.ID != id || response.Data.AccountID != "" && response.Data.AccountID != client.adsAccountID {
		return nil, platformContractError(operation, "X returned a Funding Instrument owned by another Ads Account")
	}
	if response.Data.AccountID == "" {
		response.Data.AccountID = client.adsAccountID
	}
	return &response.Data, nil
}

func validPermissions(values []string) bool {
	if len(values) == 0 || len(values) > 16 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) == 0 || len(value) > 64 {
			return false
		}
		for index := range value {
			character := value[index]
			if character != '_' && (character < 'A' || character > 'Z') {
				return false
			}
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

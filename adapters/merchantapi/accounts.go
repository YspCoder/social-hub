package merchantapi

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetMerchantAccount(ctx context.Context, options ...socialhub.CallOption) (*MerchantAccount, error) {
	const operation = "accounts.get"
	var response MerchantAccount
	path := "/accounts/v1/" + client.accountName()
	if _, err := client.getJSON(ctx, operation, path, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validMerchantAccount(client.merchantAccountID, response) {
		return nil, platformContractError(operation, "Merchant API returned a mismatched or malformed account")
	}
	return &response, nil
}

func (client *Client) ListAccountIssues(ctx context.Context, input IssueListRequest, options ...socialhub.CallOption) (TokenPage[AccountIssue], error) {
	const operation = "account_issues.list"
	if !validIssueListRequest(input) {
		return TokenPage[AccountIssue]{}, invalidArgument(operation, "page size, page token, language code, or time zone is invalid")
	}
	query := make(url.Values)
	if input.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(input.PageSize))
	}
	if input.PageToken != "" {
		query.Set("pageToken", input.PageToken)
	}
	if input.LanguageCode != "" {
		query.Set("languageCode", input.LanguageCode)
	}
	if input.TimeZone != "" {
		query.Set("timeZone", input.TimeZone)
	}
	var response struct {
		AccountIssues []AccountIssue `json:"accountIssues"`
		NextPageToken string         `json:"nextPageToken"`
	}
	path := "/accounts/v1/" + client.accountName() + "/issues"
	if _, err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return TokenPage[AccountIssue]{}, err
	}
	if len(response.AccountIssues) > effectivePageSize(input.PageSize, 50) || !validPageToken(response.NextPageToken) {
		return TokenPage[AccountIssue]{}, platformContractError(operation, "Merchant API returned invalid account issue pagination")
	}
	seen := make(map[string]struct{}, len(response.AccountIssues))
	for _, issue := range response.AccountIssues {
		if !validAccountIssue(client.merchantAccountID, issue) {
			return TokenPage[AccountIssue]{}, platformContractError(operation, "Merchant API returned a malformed account issue")
		}
		if _, found := seen[issue.Name]; found {
			return TokenPage[AccountIssue]{}, platformContractError(operation, "Merchant API returned a duplicate account issue")
		}
		seen[issue.Name] = struct{}{}
	}
	return TokenPage[AccountIssue]{
		Items: append([]AccountIssue(nil), response.AccountIssues...), NextPageToken: response.NextPageToken,
	}, nil
}

func validMerchantAccount(accountID string, value MerchantAccount) bool {
	return validAccountName(accountID, value.Name) && value.AccountID == accountID &&
		validOptionalText(value.AccountName, 4096) && validLanguageCode(value.LanguageCode) &&
		validTimeZone(value.TimeZone.ID)
}

func validAccountIssue(accountID string, value AccountIssue) bool {
	return validChildResourceName(accountID, "issues", value.Name) && validOptionalText(value.Title, 8192) &&
		validOptionalText(value.Detail, 32768) && validOptionalHTTPURL(value.DocumentationURI)
}

var _ AccountWorkflow = (*Client)(nil)

package merchantapi

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListQuotaGroups(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[QuotaGroup], error) {
	const operation = "quotas.list"
	if !validListRequest(input, 1000) {
		return TokenPage[QuotaGroup]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	var response struct {
		QuotaGroups   []QuotaGroup `json:"quotaGroups"`
		NextPageToken string       `json:"nextPageToken"`
	}
	path := "/quota/v1/" + client.accountName() + "/quotas"
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return TokenPage[QuotaGroup]{}, err
	}
	if len(response.QuotaGroups) > effectivePageSize(input.PageSize, 500) || !validPageToken(response.NextPageToken) {
		return TokenPage[QuotaGroup]{}, platformContractError(operation, "Merchant API returned invalid quota pagination")
	}
	seen := make(map[string]struct{}, len(response.QuotaGroups))
	for _, group := range response.QuotaGroups {
		if !validQuotaGroup(client.merchantAccountID, group) {
			return TokenPage[QuotaGroup]{}, platformContractError(operation, "Merchant API returned a malformed quota group")
		}
		if _, found := seen[group.Name]; found {
			return TokenPage[QuotaGroup]{}, platformContractError(operation, "Merchant API returned a duplicate quota group")
		}
		seen[group.Name] = struct{}{}
	}
	return TokenPage[QuotaGroup]{Items: append([]QuotaGroup(nil), response.QuotaGroups...), NextPageToken: response.NextPageToken}, nil
}

func (client *Client) ListProductLimits(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[AccountLimit], error) {
	const operation = "product_limits.list"
	if !validListRequest(input, 100) {
		return TokenPage[AccountLimit]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	query := listQuery(input)
	query.Set("filter", `type = "products"`)
	var response struct {
		AccountLimits []AccountLimit `json:"accountLimits"`
		NextPageToken string         `json:"nextPageToken"`
	}
	path := "/quota/v1/" + client.accountName() + "/limits"
	if _, err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return TokenPage[AccountLimit]{}, err
	}
	if len(response.AccountLimits) > effectivePageSize(input.PageSize, 100) || !validPageToken(response.NextPageToken) {
		return TokenPage[AccountLimit]{}, platformContractError(operation, "Merchant API returned invalid product-limit pagination")
	}
	for _, limit := range response.AccountLimits {
		if !validAccountLimit(client.merchantAccountID, limit) {
			return TokenPage[AccountLimit]{}, platformContractError(operation, "Merchant API returned a malformed product limit")
		}
	}
	return TokenPage[AccountLimit]{Items: append([]AccountLimit(nil), response.AccountLimits...), NextPageToken: response.NextPageToken}, nil
}

func validQuotaGroup(accountID string, value QuotaGroup) bool {
	if !validOpaqueChildResourceName(accountID, "quotas", value.Name) || !validOptionalUint(value.QuotaUsage) ||
		!validOptionalUint(value.QuotaLimit) || !validOptionalUint(value.QuotaMinuteLimit) {
		return false
	}
	for _, detail := range value.MethodDetails {
		if !validOptionalText(detail.Method, 512) || !validOptionalText(detail.SubAPI, 128) ||
			!validOptionalText(detail.Path, 1024) || !validOptionalText(detail.Version, 64) {
			return false
		}
	}
	return true
}

func validAccountLimit(accountID string, value AccountLimit) bool {
	return validChildResourceName(accountID, "limits", value.Name) && value.Products != nil &&
		value.Products.Limit != "" && validOptionalUint(value.Products.Limit) && validOptionalText(value.Products.Scope, 128)
}

var _ QuotaWorkflow = (*Client)(nil)

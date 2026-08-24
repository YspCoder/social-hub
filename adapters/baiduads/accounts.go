package baiduads

import (
	"context"

	"social-hub/pkg/socialhub"
)

var defaultAccountFields = []string{
	"userId", "balance", "pcBalance", "cost", "payment", "budget", "budgetType", "regDomain", "userStat", "userLevel",
}

func (client *Client) GetAccount(ctx context.Context, fields []string, options ...socialhub.CallOption) (*Account, error) {
	const operation = "account_get"
	if len(fields) == 0 {
		fields = defaultAccountFields
	}
	fields = appendRequiredFields(fields, "userId")
	if err := validateFields(operation, fields, 64); err != nil {
		return nil, err
	}
	var response apiEnvelope[[]Account]
	header, err := client.requestJSON(ctx, operation, "/json/sms/service/AccountService/getAccountInfo", map[string]any{
		"accountFields": fields,
	}, &response, options...)
	if err != nil {
		return nil, err
	}
	values, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	account, err := oneResult(operation, *values)
	if err != nil {
		return nil, err
	}
	if account.UserID <= 0 {
		return nil, platformContractError(operation, "Baidu Ads returned an invalid account userId")
	}
	return account, nil
}

var _ AccountWorkflow = (*Client)(nil)

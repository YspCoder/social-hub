package jinghongads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListAccounts(ctx context.Context, options ...socialhub.CallOption) (AccountList, error) {
	const operation = "accounts_list"
	var raw json.RawMessage
	if err := client.doJSON(ctx, operation, ScopeAccount, http.MethodGet, "/openapi/v1/account/profile/query", nil, &raw, options...); err != nil {
		return AccountList{}, err
	}
	result, err := decodeAccountList(raw)
	if err != nil {
		return AccountList{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return result, nil
}

func decodeAccountList(raw json.RawMessage) (AccountList, error) {
	trimmed := bytes.TrimSpace(raw)
	var result AccountList
	switch {
	case len(trimmed) > 0 && trimmed[0] == '[':
		if err := json.Unmarshal(trimmed, &result.Accounts); err != nil {
			return AccountList{}, err
		}
		result.Total = len(result.Accounts)
	case len(trimmed) > 0 && trimmed[0] == '{':
		var wire struct {
			Total    json.RawMessage     `json:"total"`
			Accounts []AdvertiserAccount `json:"accountList"`
		}
		if err := json.Unmarshal(trimmed, &wire); err != nil {
			return AccountList{}, err
		}
		total, err := decodeNonnegativeInt(wire.Total, 1_000_000)
		if err != nil {
			return AccountList{}, err
		}
		result.Accounts, result.Total = wire.Accounts, total
	default:
		return AccountList{}, errors.New("jinghongads: invalid account data")
	}
	if len(result.Accounts) > 10_000 || result.Total < len(result.Accounts) {
		return AccountList{}, errors.New("jinghongads: invalid account total")
	}
	for _, account := range result.Accounts {
		if !validID(account.ID) || !validResponseText(account.Name, 512) ||
			!validResponseText(account.CorporationName, 512) || !validResponseText(account.Type, 128) ||
			!validResponseText(account.ServiceType, 128) {
			return AccountList{}, errors.New("jinghongads: invalid account fields")
		}
	}
	return result, nil
}

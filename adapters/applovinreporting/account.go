package applovinreporting

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) AccountInfo(ctx context.Context, options ...socialhub.CallOption) (AccountInfo, error) {
	const operation = "account_info"
	callOptions, err := supportedCallOptions(operation, options)
	if err != nil {
		return AccountInfo{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, "/accountInfo", nil, nil, forwardCallOptions(callOptions)...)
	if err != nil {
		return AccountInfo{}, withOperation(err, operation)
	}
	var wire struct {
		AccountID json.RawMessage `json:"account_id"`
	}
	if err := client.api.Do(request, &wire); err != nil {
		var reporting *APIError
		if client.accountType == AccountTypeApp && errors.As(err, &reporting) && reporting.Hub != nil &&
			reporting.Hub.HTTPStatus == http.StatusForbidden && reporting.notWebAccount {
			return AccountInfo{AccountID: client.axonAccountID, AccountType: AccountTypeApp, AccountIDVerified: false}, nil
		}
		if client.accountType == AccountTypeWeb && errors.As(err, &reporting) && reporting.Hub != nil &&
			reporting.Hub.HTTPStatus == http.StatusForbidden && reporting.notWebAccount {
			return AccountInfo{}, accountMismatch(operation, "Report Key belongs to an APP account, but account_type is WEB")
		}
		return AccountInfo{}, withOperation(err, operation)
	}
	if client.accountType != AccountTypeWeb {
		return AccountInfo{}, accountMismatch(operation, "Report Key belongs to a WEB account, but account_type is APP")
	}
	id, ok := parseAccountID(wire.AccountID)
	if !ok {
		return AccountInfo{}, platformContractError(operation, "AppLovin /accountInfo returned an invalid account_id")
	}
	if id != client.axonAccountID {
		return AccountInfo{}, accountMismatch(operation, "Report Key belongs to a different WEB account_id")
	}
	return AccountInfo{AccountID: id, AccountType: AccountTypeWeb, AccountIDVerified: true}, nil
}

func parseAccountID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var text string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return "", false
		}
	} else {
		var number json.Number
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.UseNumber()
		if decoder.Decode(&number) != nil {
			return "", false
		}
		text = number.String()
		if _, err := strconv.ParseUint(text, 10, 64); err != nil {
			return "", false
		}
	}
	return text, validNumericID(text)
}

func accountMismatch(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message,
	}
}

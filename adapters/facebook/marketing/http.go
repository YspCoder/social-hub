package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) postForm(ctx context.Context, path string, form url.Values, output any, options ...socialhub.CallOption) error {
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(form.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.api.Do(request, output)
}

func setJSONForm(form url.Values, key string, value any, operation string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	form.Set(key, string(encoded))
	return nil
}

func setPositiveInt(form url.Values, key string, value int64) {
	if value > 0 {
		form.Set(key, strconv.FormatInt(value, 10))
	}
}

func addPaging(query url.Values, cursor string, limit int) {
	if cursor != "" {
		query.Set("after", cursor)
	}
	query.Set("limit", strconv.Itoa(limit))
}

func toPage[T any](response graphPage[T]) socialhub.Page[T] {
	return socialhub.Page[T]{
		Items: response.Data, NextCursor: response.Paging.Cursors.After,
		PrevCursor: response.Paging.Cursors.Before, HasMore: response.Paging.Next != "",
	}
}

func requireResponseID(operation, expected, actual string) error {
	if !validNumericID(actual) || expected != "" && actual != expected {
		return platformContractError(operation, "Meta returned a missing or mismatched resource ID")
	}
	return nil
}

func requireMutationSuccess(operation string, response successResponse) error {
	if response.Success || response.ID != "" {
		return nil
	}
	return platformContractError(operation, "Meta did not confirm the mutation")
}

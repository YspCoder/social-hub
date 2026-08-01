package line

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (c *Client) request(ctx context.Context, client *transport.Client, method, path string, query url.Values, input, output any, retryKey bool, options ...socialhub.CallOption) error {
	var body *bytes.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(method+" "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := client.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	request.Header.Del("Idempotency-Key")
	if retryKey && idempotencyKey != "" {
		if !validUUID(idempotencyKey) {
			return invalidArgument("retry_key", "idempotency key must be a UUID for LINE retry semantics")
		}
		request.Header.Set("X-Line-Retry-Key", idempotencyKey)
	}
	return client.Do(request, output)
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}

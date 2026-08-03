package strava

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) form(ctx context.Context, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := client.api.Do(request, output); err != nil {
		return err
	}
	return nil
}

func validResourceID(value string) bool {
	if strings.TrimSpace(value) != value || value == "" {
		return false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed > 0
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validText(value string, maximum int, allowEmpty bool) bool {
	if len(value) > maximum || !allowEmpty && strings.TrimSpace(value) == "" {
		return false
	}
	for _, character := range value {
		if character == 0 || character == 0x7f {
			return false
		}
	}
	return true
}

func boolForm(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

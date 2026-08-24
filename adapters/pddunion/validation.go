package pddunion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Pinduoduo assigns request IDs; caller request IDs are not supported")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "Duoduo Jinbao query and link workflows do not define idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Duoduo Jinbao method")
	}
	return nil
}

func validGatewayURL(value string) bool {
	return value == defaultBaseURL
}

func splitGatewayURL(value string) (string, string, error) {
	parsed, err := url.Parse(value)
	if err != nil || !validGatewayURL(value) {
		return "", "", fmt.Errorf("invalid Pinduoduo gateway URL")
	}
	return parsed.Scheme + "://" + parsed.Host, parsed.Path, nil
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validPID(value string) bool {
	if !validOpaque(value, 128) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || character == ',' {
			return false
		}
	}
	return true
}

func validCustomParameters(value string) bool {
	if value == "" {
		return true
	}
	trimmed := bytes.TrimSpace([]byte(value))
	if len(trimmed) == 0 || len(trimmed) > 64 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(trimmed, &object) == nil && object != nil
}

func validOrderRange(start, end time.Time) bool {
	return !start.IsZero() && !end.IsZero() && start.Unix() > 0 && !end.Before(start)
}

func setString(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func setInt(values url.Values, key string, value int64) {
	if value != 0 {
		values.Set(key, fmt.Sprintf("%d", value))
	}
}

func setBool(values url.Values, key string, value *bool) {
	if value != nil {
		values.Set(key, fmt.Sprintf("%t", *value))
	}
}

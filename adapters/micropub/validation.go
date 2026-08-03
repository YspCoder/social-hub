package micropub

import (
	"encoding/json"
	"mime"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	maxTextBytes               = 8 << 20
	maxPropertyCount           = 1024
	maxValuesPerProperty       = 10_000
	maxUploadBytes       int64 = 8 << 30
)

var typedPropertyNames = map[string]struct{}{
	"name": {}, "summary": {}, "content": {}, "published": {}, "category": {}, "location": {},
	"in-reply-to": {}, "like-of": {}, "repost-of": {}, "photo": {}, "video": {}, "audio": {}, "mp-syndicate-to": {},
}

func validText(value string, allowEmpty bool) bool {
	return (allowEmpty || strings.TrimSpace(value) != "") && len(value) <= maxTextBytes && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validAbsoluteURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validPropertyName(value string) bool {
	if value == "" || len(value) > 128 || strings.HasPrefix(strings.ToLower(value), "mp-") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e || character == '&' || character == '=' || character == '[' || character == ']' {
			return false
		}
	}
	switch value {
	case "access_token", "h", "action", "url":
		return false
	default:
		return true
	}
}

func validateRawProperties(operation string, properties map[string][]json.RawMessage, rejectTyped bool) error {
	if len(properties) > maxPropertyCount {
		return invalidArgument(operation, "too many Micropub properties")
	}
	for name, values := range properties {
		if !validPropertyName(name) {
			return invalidArgument(operation, "property name is invalid or reserved")
		}
		if rejectTyped {
			if _, exists := typedPropertyNames[name]; exists {
				return invalidArgument(operation, "extra_properties must not overwrite typed fields")
			}
		}
		if len(values) == 0 || len(values) > maxValuesPerProperty {
			return invalidArgument(operation, "property value arrays must be non-empty and bounded")
		}
		for _, value := range values {
			if len(value) == 0 || len(value) > maxTextBytes || !json.Valid(value) {
				return invalidArgument(operation, "property values must contain valid bounded JSON")
			}
		}
	}
	return nil
}

func validateURLList(operation string, values []string) error {
	if len(values) > maxValuesPerProperty {
		return invalidArgument(operation, "too many URL values")
	}
	for _, value := range values {
		if !validAbsoluteURL(value) {
			return invalidArgument(operation, "URL properties must be absolute HTTP(S) URLs")
		}
	}
	return nil
}

func validFilename(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 255 && value != "." && value != ".." &&
		!strings.ContainsAny(value, "\x00\r\n/\\")
}

func validMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.Contains(mediaType, "/") && len(value) <= 255
}

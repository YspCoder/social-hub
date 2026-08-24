package jinghongads

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPErrorAt(status, header, body, clock.Now())
	}
}

func decodeHTTPErrorAt(status int, header http.Header, body []byte, now time.Time) error {
	var payload struct {
		Code     json.RawMessage `json:"code"`
		Error    json.RawMessage `json:"error"`
		SubError json.RawMessage `json:"sub_error"`
	}
	_ = json.Unmarshal(body, &payload)
	platformCode := firstNonEmpty(scalarCode(payload.Code), scalarCode(payload.Error))
	if subError := scalarCode(payload.SubError); subError != "" {
		platformCode = joinPlatformCodes(platformCode, subError)
	}
	code, class := classifyHTTPError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 128), PlatformMessage: "Jinghong Marketing API rejected the request",
		RequestID:  firstSafeHeader(header, 256, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func businessError(operation string, status int, header http.Header, platformCode string, now time.Time) error {
	code, class := classifyBusinessCode(platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 128),
		PlatformMessage: "Jinghong Marketing API returned an unsuccessful business code",
		RequestID:       firstSafeHeader(header, 256, "X-Request-ID", "X-Correlation-ID"),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func classifyBusinessCode(code string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case "1000009992":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "1000015", "1013001", "1013002", "1000020000", "1000020001":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "1000003", "1000014", "1001024", "1001025", "1001029", "1001031", "1001032":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "1000001", "1000007", "1000008", "1000009", "1000010", "1000011", "1000012", "1000013", "1000016",
		"1001003", "1001004", "1001012", "1001036", "1002002", "1003061", "1003063", "1004009", "1004044", "1005002", "1005005":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "1001001", "1001002", "1002003", "1002016", "1004001", "1004002", "1005008", "1005010", "1005011", "1005025":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "1000002", "1000004", "1000005", "1000006", "1000017", "1000018", "1000020":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case http.StatusTooManyRequests:
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, credentials ...string) error {
	var sanitized error
	if clean := sanitizeCause(cause); clean != nil {
		causeMessage := clean.Error()
		for _, credential := range credentials {
			causeMessage = redactExact(causeMessage, credential)
		}
		causeMessage = boundedMessage(redactSensitive(causeMessage), 1024)
		if causeMessage != "" {
			sanitized = errors.New(causeMessage)
		}
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: sanitized,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func withOperation(err error, operation string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
	}
	return err
}

func scalarCode(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || len(trimmed) > 130 {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var value string
		if json.Unmarshal(raw, &value) == nil && validPlatformCode(value) {
			return value
		}
		return ""
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil && validPlatformCode(number.String()) {
		return number.String()
	}
	return ""
}

func decodeID(raw json.RawMessage) (string, error) {
	value := scalarCode(raw)
	if !validID(value) {
		return "", errors.New("jinghongads: invalid or missing identifier")
	}
	return value, nil
}

func decodeOptionalID(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	return decodeID(raw)
}

func decodeNonnegativeInt(raw json.RawMessage, maximum int) (int, error) {
	value := scalarCode(raw)
	if value == "" {
		return 0, errors.New("jinghongads: missing integer")
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > maximum {
		return 0, errors.New("jinghongads: invalid integer")
	}
	return parsed, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = safeHeaderValue(value, 128)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	delay := when.Sub(now)
	if delay < 0 || delay > 24*time.Hour {
		return 0
	}
	return delay
}

func firstSafeHeader(header http.Header, maximum int, names ...string) string {
	for _, name := range names {
		if value := safeHeaderValue(header.Get(name), maximum); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedMessage(value string, maximum int) string {
	if maximum <= 0 || !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func safeHeaderValue(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func validPlatformCode(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := range value {
		character := value[index]
		if character != '_' && character != '-' && character != '.' &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func joinPlatformCodes(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + "/" + right
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func redactSensitive(value string) string {
	markers := []string{"authorization", "access_token", "refresh_token", "client_secret", "bearer", "token"}
	for _, marker := range markers {
		for cursor := 0; cursor < len(value); {
			index := strings.Index(strings.ToLower(value[cursor:]), marker)
			if index < 0 {
				break
			}
			index += cursor
			start := index + len(marker)
			for start < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[start])) {
				start++
			}
			if start == index+len(marker) {
				cursor = start
				continue
			}
			end := start
			for end < len(value) && !strings.ContainsRune("\r\n,;}&\"'", rune(value[end])) &&
				(marker == "authorization" || !strings.ContainsRune(" \t", rune(value[end]))) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}

func redactExact(value, credential string) string {
	if credential == "" {
		return value
	}
	return strings.ReplaceAll(value, credential, "[REDACTED]")
}

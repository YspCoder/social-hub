package petalads

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func decodeHTTPError(status int, header http.Header, body []byte) error {
	return decodeHTTPErrorAt(status, header, body, time.Now())
}

func newHTTPErrorDecoder(clock socialhub.Clock, requestIDValues ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPErrorAt(status, header, body, clock.Now(), requestIDValues...)
	}
}

func decodeHTTPErrorAt(status int, header http.Header, body []byte, now time.Time, requestIDValues ...string) error {
	var payload struct {
		Code     json.RawMessage `json:"code"`
		Error    json.RawMessage `json:"error"`
		SubError json.RawMessage `json:"sub_error"`
	}
	_ = json.Unmarshal(body, &payload)
	platformCode := firstNonEmpty(numericCode(payload.Code), numericCode(payload.Error))
	if subError := numericCode(payload.SubError); subError != "" {
		platformCode = strings.Trim(platformCode+"/"+subError, "/")
	}
	code, class := classifyHTTPError(status)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedOpaque(platformCode, 128), PlatformMessage: "Petal Ads rejected the request",
		RequestID:  responseRequestID(header, requestIDValues...),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func businessError(operation string, status int, header http.Header, platformCode string, now time.Time, requestIDValues ...string) error {
	code, class := classifyBusinessCode(platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedOpaque(platformCode, 128),
		PlatformMessage: "Petal Ads returned an unsuccessful business code",
		RequestID:       responseRequestID(header, requestIDValues...),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
}

func classifyBusinessCode(code string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch code {
	case "1000009992":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "1000015":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "1000003", "1000014", "1001024", "1001025", "1001029", "1001031", "1001032":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "1000001", "1000007", "1000008", "1000009", "1000010", "1000011", "1000012", "1000013", "1000016",
		"1001003", "1001004", "1001012", "1001036", "1002002", "1003061", "1003063", "1004009", "1004044", "1005002", "1005005":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "1001001", "1001002", "1001041", "1002003", "1002016", "1004001", "1004002", "1005008", "1005010", "1005011", "1005025":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "1000002", "1000004", "1000005", "1000006", "1000018", "1000020":
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
		if json.Unmarshal(raw, &value) == nil && validOpaque(value, 128) {
			return value
		}
		return ""
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return ""
}

func numericCode(raw json.RawMessage) string {
	value := scalarCode(raw)
	if value == "" {
		return ""
	}
	start := 0
	if value[0] == '-' {
		start = 1
	}
	if start == len(value) {
		return ""
	}
	for index := start; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return ""
		}
	}
	return value
}

func decodeID(raw json.RawMessage) (string, error) {
	value := scalarCode(raw)
	if !validID(value) {
		return "", errors.New("petalads: invalid or missing identifier")
	}
	return value, nil
}

func decodeNonnegativeInt(raw json.RawMessage, maximum int) (int, error) {
	value := scalarCode(raw)
	if value == "" {
		return 0, errors.New("petalads: missing integer")
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || parsed > maximum {
		return 0, errors.New("petalads: invalid integer")
	}
	return parsed, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
		return 0
	}
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
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

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func responseRequestID(header http.Header, blockedValues ...string) string {
	value := boundedOpaque(firstHeader(header, "DGW.id", "oauth_trace_id"), 256)
	for _, blocked := range blockedValues {
		if blocked != "" && strings.Contains(value, blocked) {
			return ""
		}
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boundedOpaque(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if !validOpaque(value, maximum) {
		return ""
	}
	return value
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

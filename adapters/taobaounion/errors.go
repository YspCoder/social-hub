package taobaounion

import (
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

type topErrorResponse struct {
	Code      json.RawMessage `json:"code"`
	SubCode   string          `json:"sub_code"`
	RequestID string          `json:"request_id"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var root struct {
			Error *topErrorResponse `json:"error_response"`
		}
		_ = json.Unmarshal(body, &root)
		if root.Error != nil {
			return topErrorValue("http", status, header, *root.Error, clock.Now())
		}
		code, class := classifyTOPError(status, "", "")
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "TOP gateway rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		setApprovalURL(result)
		return result
	}
}

func topErrorValue(operation string, status int, header http.Header, response topErrorResponse, now time.Time) error {
	platformCode := scalarString(response.Code)
	if response.SubCode != "" {
		platformCode = strings.Trim(platformCode+"/"+response.SubCode, "/")
	}
	code, class := classifyTOPError(status, scalarString(response.Code), response.SubCode)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "TOP returned an error response",
		RequestID:       boundedMessage(firstNonEmpty(response.RequestID, firstHeader(header, "X-Request-ID", "X-Correlation-ID")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func setApprovalURL(err *socialhub.Error) {
	if err.Code == socialhub.CodeUnauthenticated || err.Code == socialhub.CodePermissionDenied ||
		err.Code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = documentationURL
	}
}

func classifyTOPError(status int, platformCode, subCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.TrimSpace(subCode))
	switch {
	case strings.Contains(normalized, "call-limit"), strings.Contains(normalized, "frequency"),
		strings.Contains(normalized, "flow-control"), strings.Contains(normalized, "too-many"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "session"), strings.Contains(normalized, "signature"),
		strings.Contains(normalized, "appkey"), strings.Contains(normalized, "app-key"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "permission"), strings.Contains(normalized, "authorize"),
		strings.Contains(normalized, "package-limit"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case strings.Contains(normalized, "missing-parameter"), strings.Contains(normalized, "invalid-parameter"),
		strings.Contains(normalized, "illegal-argument"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case strings.Contains(normalized, "not-found"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(normalized, "service-unavailable"), strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "remote-connection"), strings.Contains(normalized, "isp-error"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch platformCode {
	case "7", "43":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "10", "15":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "11", "12", "13":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "24", "25", "26", "27", "28", "29":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "21", "22", "23", "30", "31", "32", "33", "34", "40", "41":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusRequestEntityTooLarge,
		http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
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
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		Cause: sanitizeCause(cause),
	}
}

func authenticationError(operation, message string, cause error, credential string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		cause = errors.New(boundedMessage(redactExact(redactSensitive(clean.Error()), credential), 1024))
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: cause, ApprovalURL: documentationURL,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: boundedMessage(message, 1024),
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

func scalarString(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "null" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var decoded string
		if json.Unmarshal(value, &decoded) == nil {
			return decoded
		}
		return ""
	}
	return trimmed
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
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

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := header.Get(name); value != "" {
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
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactExact(value, credential string) string {
	if credential == "" {
		return value
	}
	return strings.ReplaceAll(value, credential, "[REDACTED]")
}

func redactSensitive(value string) string {
	for _, key := range []string{"session", "access_token", "app_secret", "client_secret", "authorization"} {
		cursor := 0
		for {
			start := strings.Index(strings.ToLower(value[cursor:]), key)
			if start < 0 {
				break
			}
			start += cursor
			valueStart := start + len(key)
			for valueStart < len(value) && strings.ContainsRune(" \t:=\"'", rune(value[valueStart])) {
				valueStart++
			}
			if valueStart == start+len(key) {
				cursor = valueStart
				continue
			}
			valueEnd := valueStart
			for valueEnd < len(value) && !strings.ContainsRune(" \t\r\n,;&\"'", rune(value[valueEnd])) {
				valueEnd++
			}
			value = value[:valueStart] + "[REDACTED]" + value[valueEnd:]
			cursor = valueStart + len("[REDACTED]")
		}
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

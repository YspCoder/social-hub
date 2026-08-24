package aliexpressaffiliate

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
	Type      string          `json:"type"`
	Code      json.RawMessage `json:"code"`
	Message   string          `json:"message"`
	Msg       string          `json:"msg"`
	RequestID string          `json:"request_id"`
	TraceID   string          `json:"_trace_id_"`
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
			HTTPStatus: status, PlatformMessage: "AliExpress gateway rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		setApprovalURL(result)
		return result
	}
}

func topErrorValue(operation string, status int, header http.Header, response topErrorResponse, now time.Time) error {
	platformCode := scalarString(response.Code)
	message := firstNonEmpty(response.Message, response.Msg, "AliExpress returned an error response")
	code, class := classifyTOPError(status, platformCode, message)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "AliExpress returned an error response",
		RequestID: boundedMessage(firstNonEmpty(
			response.RequestID, response.TraceID, firstHeader(header, "X-Request-ID", "X-Correlation-ID"),
		), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func businessError(operation, platformCode, message, requestID string) error {
	code, class := classifyTOPError(http.StatusOK, platformCode, message)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: http.StatusOK, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "AliExpress Affiliate method returned an error response",
		RequestID:       boundedMessage(requestID, 256),
	}
	setApprovalURL(result)
	return result
}

func setApprovalURL(err *socialhub.Error) {
	if err.Code == socialhub.CodeUnauthenticated || err.Code == socialhub.CodePermissionDenied ||
		err.Code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = approvalURL
	}
}

func classifyTOPError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.TrimSpace(platformCode + " " + message))
	switch {
	case strings.Contains(normalized, "call limit"), strings.Contains(normalized, "call-limit"),
		strings.Contains(normalized, "frequency"), strings.Contains(normalized, "flow control"),
		strings.Contains(normalized, "too many"), strings.Contains(normalized, "throttl"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "signature"), strings.Contains(normalized, "signcheck"),
		strings.Contains(normalized, "appkey"), strings.Contains(normalized, "app key"),
		strings.Contains(normalized, "session"), strings.Contains(normalized, "access token"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "permission"), strings.Contains(normalized, "authorize"),
		strings.Contains(normalized, "authorization"), strings.Contains(normalized, "isv not allowed"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case strings.Contains(normalized, "missingparameter"), strings.Contains(normalized, "missing parameter"),
		strings.Contains(normalized, "invalidparameter"), strings.Contains(normalized, "invalid parameter"),
		strings.Contains(normalized, "illegal argument"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case strings.Contains(normalized, "not found"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(normalized, "service unavailable"), strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "system error"), strings.Contains(normalized, "remote connection"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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
	case http.StatusRequestTimeout:
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
		Cause:           sanitizeCredentialCause(cause, secrets...),
		ApprovalURL:     approvalURL,
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

func redactExact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
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

func sanitizeCredentialCause(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	clean := sanitizeCause(err)
	return errors.New(boundedMessage(redactExact(redactSensitive(clean.Error()), secrets...), 1024))
}

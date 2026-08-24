package pddunion

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

type pddErrorResponse struct {
	ErrorCode json.RawMessage `json:"error_code"`
	ErrorMsg  string          `json:"error_msg"`
	SubCode   string          `json:"sub_code"`
	SubMsg    string          `json:"sub_msg"`
	RequestID string          `json:"request_id"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var root struct {
			Error *pddErrorResponse `json:"error_response"`
		}
		_ = json.Unmarshal(body, &root)
		if root.Error != nil {
			return pddErrorValue("http", status, header, *root.Error, clock.Now())
		}
		code, class := classifyPDDError(status, "", "", "")
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "Pinduoduo gateway rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		setApprovalURL(result)
		return result
	}
}

func pddErrorValue(operation string, status int, header http.Header, response pddErrorResponse, now time.Time) error {
	platformCode := scalarString(response.ErrorCode)
	if response.SubCode != "" {
		platformCode = strings.Trim(platformCode+"/"+response.SubCode, "/")
	}
	message := firstNonEmpty(response.SubMsg, response.ErrorMsg, "Pinduoduo returned an error response")
	code, class := classifyPDDError(status, scalarString(response.ErrorCode), response.SubCode, message)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "Pinduoduo returned an error response",
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

func classifyPDDError(status int, platformCode, subCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToLower(strings.TrimSpace(subCode + " " + message))
	switch platformCode {
	case "50000":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "10010", "20000":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "10000", "43002", "43003", "43004":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "50001":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	switch {
	case strings.Contains(normalized, "qps"), strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "too many"), strings.Contains(normalized, "frequent"),
		strings.Contains(normalized, "traffic limit"), strings.Contains(normalized, "限流"),
		strings.Contains(normalized, "频繁"), strings.Contains(normalized, "流量限制"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "system error"), strings.Contains(normalized, "系统异常"),
		strings.Contains(normalized, "服务异常"), strings.Contains(normalized, "稍后重试"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case strings.Contains(normalized, "access_token"), strings.Contains(normalized, "client_id"),
		strings.Contains(normalized, "签名"), strings.Contains(normalized, "signature"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "权限"), strings.Contains(normalized, "授权"),
		strings.Contains(normalized, "白名单"), strings.Contains(normalized, "备案"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case strings.Contains(normalized, "参数"), strings.Contains(normalized, "格式"),
		strings.Contains(normalized, "timestamp"):
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
	for _, key := range []string{"access_token", "client_secret", "authorization"} {
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

package vipunion

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

type vipErrorResponse struct {
	ReturnCode    json.RawMessage `json:"returnCode"`
	ReturnMessage string          `json:"returnMessage"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var response vipErrorResponse
		if json.Unmarshal(body, &response) == nil && scalarString(response.ReturnCode) != "" {
			return vipErrorValue("http", status, header, response, "", clock.Now())
		}
		code, class := classifyVIPError(status, "", string(body))
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "Vipshop gateway rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		setApprovalURL(result)
		return result
	}
}

func vipErrorValue(
	operation string,
	status int,
	header http.Header,
	response vipErrorResponse,
	requestID string,
	now time.Time,
) error {
	platformCode := scalarString(response.ReturnCode)
	message := firstNonEmpty(response.ReturnMessage, "Vipshop returned an error response")
	code, class := classifyVIPError(status, platformCode, message)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "Vipshop returned an error response",
		RequestID: boundedMessage(firstNonEmpty(
			requestID, firstHeader(header, "X-Request-ID", "X-Correlation-ID"),
		), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
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

func classifyVIPError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "1000", "1001", "2002":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "1002", "1003", "1004", "1005", "1006":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "1007", "1010", "1013":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "1008":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "1009", "1011", "1014":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "1012":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "2001":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "频率"), strings.Contains(normalized, "限流"),
		strings.Contains(normalized, "too many"), strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "qps"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalized, "系统异常"), strings.Contains(normalized, "服务异常"),
		strings.Contains(normalized, "稍后重试"), strings.Contains(normalized, "system error"),
		strings.Contains(normalized, "database"), strings.Contains(normalized, "timeout"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case strings.Contains(normalized, "access token"), strings.Contains(normalized, "accesstoken"),
		strings.Contains(normalized, "appkey"), strings.Contains(normalized, "签名"),
		strings.Contains(normalized, "signature"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalized, "权限"), strings.Contains(normalized, "授权"),
		strings.Contains(normalized, "白名单"), strings.Contains(normalized, "未注册"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case strings.Contains(normalized, "参数"), strings.Contains(normalized, "格式"):
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

func authenticationError(operation, message string, cause error, sensitive ...string) error {
	if cause != nil {
		clean := sanitizeCause(cause)
		causeMessage := redactSensitive(clean.Error())
		for _, value := range sensitive {
			causeMessage = redactExact(causeMessage, value)
		}
		cause = errors.New(boundedMessage(causeMessage, 1024))
	}
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024), Cause: cause, ApprovalURL: approvalURL,
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func platformContractError(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: boundedMessage(message, 1024),
	}
}

func withOperation(err error, operation, requestID string) error {
	if err == nil {
		return nil
	}
	var hub *socialhub.Error
	if errors.As(err, &hub) {
		hub.Op = operation
		if hub.RequestID == "" {
			hub.RequestID = boundedMessage(requestID, 256)
		}
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

func redactExact(value, sensitive string) string {
	if sensitive == "" {
		return value
	}
	return strings.ReplaceAll(value, sensitive, "[REDACTED]")
}

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "accesstoken", "appsecret", "authorization"} {
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

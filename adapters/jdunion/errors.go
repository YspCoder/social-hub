package jdunion

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

type jdErrorResponse struct {
	Code          json.RawMessage `json:"code"`
	Message       string          `json:"message"`
	ErrorMessage  string          `json:"errorMessage"`
	ErrorSolution string          `json:"errorSolution"`
	Chinese       string          `json:"zh_desc"`
	English       string          `json:"en_desc"`
	RequestID     string          `json:"requestId"`
}

func (response jdErrorResponse) code() string { return scalarString(response.Code) }

func decodeJDError(data []byte) (jdErrorResponse, error) {
	decoded, err := decodeObjectOrString(data)
	if err != nil {
		return jdErrorResponse{}, err
	}
	var response jdErrorResponse
	if err := json.Unmarshal(decoded, &response); err != nil {
		return jdErrorResponse{}, err
	}
	return response, nil
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var root map[string]json.RawMessage
		if json.Unmarshal(body, &root) == nil {
			if nested, found := root["error_response"]; found && hasJSONValue(nested) {
				if response, err := decodeJDError(nested); err == nil {
					return jdErrorValue("http", status, header, response, clock.Now())
				}
			}
			if response, err := decodeJDError(body); err == nil && response.code() != "" {
				return jdErrorValue("http", status, header, response, clock.Now())
			}
		}
		code, class := classifyJDError(status, "", "")
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName,
			HTTPStatus: status, PlatformMessage: "JD gateway rejected the request",
			RequestID:  boundedMessage(firstHeader(header, "X-Request-ID", "X-Correlation-ID"), 256),
			RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
		setApprovalURL(result)
		return result
	}
}

func jdErrorValue(operation string, status int, header http.Header, response jdErrorResponse, now time.Time) error {
	platformCode := response.code()
	message := firstNonEmpty(response.ErrorMessage, response.Message, response.Chinese, response.English, response.ErrorSolution, "JD returned an error response")
	code, class := classifyJDError(status, platformCode, message)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: "JD returned an error response",
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

func classifyJDError(status int, platformCode, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalizedMessage := strings.ToLower(strings.TrimSpace(message))
	switch platformCode {
	case "429":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "500", "2002500", "2002408", "2001609", "2001504", "2001904", "2001910", "2001912":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "2002401":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "403", "408", "601", "2001403", "2002403", "2001602", "2001603", "2001920", "2001928", "2001701", "2001702":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "202", "400", "401", "1002021", "1002024", "2001208", "2001212", "2001230", "2002400", "2002452", "2002453", "2002499":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch {
	case strings.Contains(normalizedMessage, "超频"), strings.Contains(normalizedMessage, "rate limit"),
		strings.Contains(normalizedMessage, "too many"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(normalizedMessage, "服务降级"), strings.Contains(normalizedMessage, "系统异常"),
		strings.Contains(normalizedMessage, "稍后重试"), strings.Contains(normalizedMessage, "temporarily"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case strings.Contains(normalizedMessage, "token") && (strings.Contains(normalizedMessage, "非法") || strings.Contains(normalizedMessage, "invalid")):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(normalizedMessage, "权限"), strings.Contains(normalizedMessage, "协议"),
		strings.Contains(normalizedMessage, "限制调用"), strings.Contains(normalizedMessage, "账户状态异常"):
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case strings.Contains(normalizedMessage, "参数"), strings.Contains(normalizedMessage, "格式"),
		strings.Contains(normalizedMessage, "时间范围"), strings.Contains(normalizedMessage, "缩小查询"):
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
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		if status == http.StatusTooManyRequests {
			return socialhub.CodeRateLimited, socialhub.ClassRetryable
		}
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
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
	for _, key := range []string{"access_token", "app_secret", "client_secret", "authorization"} {
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

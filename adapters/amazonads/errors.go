package amazonads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiErrorEnvelope
	_ = json.Unmarshal(body, &response)
	platformCode := firstNonEmpty(response.Code, response.Error.Code)
	message := firstNonEmpty(response.Message, response.Error.Message, response.Details)
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-amzn-RequestId"), header.Get("x-amz-request-id"), header.Get("x-request-id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	upper := strings.ToUpper(platformCode)
	switch {
	case strings.Contains(upper, "THROTTL"), strings.Contains(upper, "RATE_LIMIT"), strings.Contains(upper, "TOO_MANY_REQUESTS"):
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case strings.Contains(upper, "UNAUTHORIZED"), strings.Contains(upper, "INVALID_TOKEN"), strings.Contains(upper, "INVALID_GRANT"):
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case strings.Contains(upper, "FORBIDDEN"), strings.Contains(upper, "PERMISSION"), strings.Contains(upper, "NOT_AUTHORIZED"):
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case strings.Contains(upper, "NOT_FOUND"):
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case strings.Contains(upper, "CONFLICT"), strings.Contains(upper, "DUPLICATE"):
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case strings.Contains(upper, "INVALID"), strings.Contains(upper, "MALFORMED"), strings.Contains(upper, "BAD_REQUEST"):
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case strings.Contains(upper, "INTERNAL"), strings.Contains(upper, "UNAVAILABLE"), strings.Contains(upper, "TIMEOUT"):
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
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

func mutationError(operation string, status int, header http.Header, failure mutationFailure) error {
	platformCode, message := "", "Amazon Ads rejected a mutation item"
	if len(failure.Errors) > 0 {
		platformCode = failure.Errors[0].Type
		if len(failure.Errors[0].Value) > 0 {
			message = string(failure.Errors[0].Value)
		}
	}
	code, class := classifyError(status, platformCode)
	if code == socialhub.CodePlatformError {
		code = socialhub.CodeInvalidArgument
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: boundedMessage(platformCode, 256),
		PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("x-amzn-RequestId"), header.Get("x-amz-request-id"), header.Get("x-request-id")), 256),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: platformName, Product: productName, Op: operation, Cause: cause}
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

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	if parsed, err := http.ParseTime(value); err == nil {
		delay := time.Until(parsed)
		if delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return 0
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, marker := range []string{"access_token", "refresh_token", "client_secret", "authorization"} {
		cursor := 0
		for {
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
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;}&\"'", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
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

package ads

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiError struct {
	Parameter string          `json:"parameter"`
	Details   string          `json:"details"`
	Code      string          `json:"code"`
	Value     json.RawMessage `json:"value"`
	Message   string          `json:"message"`
}

type errorEnvelope struct {
	Errors []apiError `json:"errors"`
	Error  string     `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		return decodeHTTPError(status, header, body, clock.Now())
	}
}

func decodeHTTPError(status int, header http.Header, body []byte, now time.Time) error {
	var envelope errorEnvelope
	_ = json.Unmarshal(body, &envelope)
	platformCode, message := "", envelope.Error
	if len(envelope.Errors) > 0 {
		platformCode = envelope.Errors[0].Code
		message = firstNonEmpty(envelope.Errors[0].Message, envelope.Errors[0].Details, message)
	}
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: boundedMessage(platformCode, 256), PlatformMessage: boundedMessage(redactSensitive(message), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-request-id"), header.Get("x-transaction-id")), 256),
		RetryAfter: retryDelay(header, now),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	upper := strings.ToUpper(platformCode)
	if status == http.StatusTooManyRequests || upper == "TOO_MANY_REQUESTS" || upper == "TWEET_RATE_LIMIT_EXCEEDED" {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	}
	if upper == "UNAUTHORIZED_CLIENT_APPLICATION" || upper == "FEATURE_NOT_AVAILABLE" {
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	}
	if status == http.StatusUnauthorized || upper == "UNAUTHORIZED_ACCESS" {
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	}
	if strings.HasSuffix(upper, "_NOT_FOUND") || status == http.StatusNotFound {
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	if upper == "CANCELLED_REQUEST" || upper == "LOCK_ACQUISITION_TIMEOUT" || status == http.StatusRequestTimeout {
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusForbidden:
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusConflict:
		return socialhub.CodeConflict, socialhub.ClassPermanent
	default:
		if status >= 500 {
			return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func retryDelay(header http.Header, now time.Time) time.Duration {
	for _, name := range []string{"x-account-rate-limit-reset", "x-rate-limit-reset"} {
		reset, err := strconv.ParseInt(strings.TrimSpace(header.Get(name)), 10, 64)
		if err == nil && reset > 0 {
			delay := time.Unix(reset, 0).Sub(now)
			if delay > 0 && delay <= 24*time.Hour {
				return delay
			}
		}
	}
	value := strings.TrimSpace(header.Get("Retry-After"))
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 && seconds <= float64((24*time.Hour)/time.Second) {
		return time.Duration(seconds * float64(time.Second))
	}
	if parsed, err := http.ParseTime(value); err == nil {
		delay := parsed.Sub(now)
		if delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return 0
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

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func redactSensitive(value string) string {
	for _, marker := range []string{"oauth_token_secret", "oauth_token", "consumer_secret", "client_secret", "access_token", "authorization"} {
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

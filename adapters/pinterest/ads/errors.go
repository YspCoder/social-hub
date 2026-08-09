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

type pinterestError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response pinterestError
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Code, response.Message)
	platformCode := ""
	if response.Code != 0 {
		platformCode = strconv.Itoa(response.Code)
	}
	retryAfter := parseRetryAfter(header.Get("Retry-After"))
	if retryAfter == 0 && status == http.StatusTooManyRequests {
		retryAfter = parseDurationSeconds(header.Get("x-ratelimit-reset"))
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(redactSensitive(response.Message), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("x-pinterest-rid"), header.Get("x-request-id")), 256),
		RetryAfter: retryAfter,
	}
}

func classifyError(status, platformCode int, message string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	upper := strings.ToUpper(message)
	if strings.Contains(upper, "RATE LIMIT") || strings.Contains(upper, "THROTTL") {
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
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
		if platformCode == 2 {
			return socialhub.CodeNotFound, socialhub.ClassPermanent
		}
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func batchItemError(operation string, status int, header http.Header, exception batchException) error {
	code, class := classifyError(status, exception.Code, exception.Message)
	if code == socialhub.CodePlatformError || code == socialhub.CodeNotFound && exception.Code != 2 {
		code, class = socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	platformCode := ""
	if exception.Code != 0 {
		platformCode = strconv.Itoa(exception.Code)
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode,
		PlatformMessage: boundedMessage(redactSensitive(exception.Message), 512),
		RequestID:       boundedMessage(firstNonEmpty(header.Get("x-pinterest-rid"), header.Get("x-request-id")), 256),
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
	if duration := parseDurationSeconds(value); duration > 0 {
		return duration
	}
	if parsed, err := http.ParseTime(strings.TrimSpace(value)); err == nil {
		delay := time.Until(parsed)
		if delay > 0 && delay <= 24*time.Hour {
			return delay
		}
	}
	return 0
}

func parseDurationSeconds(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 || seconds > float64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
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

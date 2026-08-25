package searchads360

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiErrorEnvelope struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			Errors []struct {
				ErrorCode map[string]string `json:"errorCode"`
			} `json:"errors"`
		} `json:"details"`
	} `json:"error"`
}

func newHTTPErrorDecoder(clock socialhub.Clock, redactions ...string) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var response apiErrorEnvelope
		_ = json.Unmarshal(body, &response)
		platformStatus := validPlatformCode(response.Error.Status)
		platformCode := platformStatus
		for _, detail := range response.Error.Details {
			if len(detail.Errors) == 0 {
				continue
			}
			failure := detail.Errors[0]
			platformCode = firstNonEmpty(firstErrorCode(failure.ErrorCode), platformCode)
			break
		}
		code, class := classifyError(status, platformStatus, platformCode)
		return &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, HTTPStatus: status,
			PlatformCode: platformCode, PlatformMessage: "Search Ads 360 API request failed",
			RequestID: safeRequestID(header, redactions), RetryAfter: parseRetryAfter(header.Get("Retry-After"), clock.Now()),
		}
	}
}

func firstErrorCode(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := validPlatformCode(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func classifyError(status int, platformStatus, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	normalized := strings.ToUpper(strings.TrimSpace(platformCode))
	switch normalized {
	case "RESOURCE_EXHAUSTED", "RATE_EXCEEDED", "QUOTA_EXCEEDED", "RESOURCE_TEMPORARILY_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "DAILY_LIMIT_EXCEEDED":
		return socialhub.CodeRateLimited, socialhub.ClassUserAction
	case "UNAUTHENTICATED", "AUTHENTICATION_ERROR":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED", "AUTHORIZATION_ERROR", "CUSTOMER_NOT_ENABLED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "INTERNAL_ERROR", "SERVER_ERROR", "TEMPORARILY_UNAVAILABLE":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch strings.ToUpper(strings.TrimSpace(platformStatus)) {
	case "INVALID_ARGUMENT", "FAILED_PRECONDITION", "OUT_OF_RANGE":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "RESOURCE_EXHAUSTED":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "UNAUTHENTICATED":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "PERMISSION_DENIED":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "ALREADY_EXISTS", "ABORTED":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
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
	case http.StatusRequestTimeout:
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

func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return 0
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return 0
		}
	}
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil && seconds <= uint64((24*time.Hour)/time.Second) {
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

func boundedMessage(value string, maximum int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func validPlatformCode(value string) string {
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return ""
	}
	return value
}

func safeRequestID(header http.Header, redactions []string) string {
	value := strings.TrimSpace(header.Get("request-id"))
	if !validOpaque(value, 256) {
		return ""
	}
	for _, redaction := range redactions {
		if value == redaction {
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

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

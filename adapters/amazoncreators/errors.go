package amazoncreators

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

type httpErrorPayload struct {
	Type              string          `json:"type"`
	Reason            string          `json:"reason"`
	RetryAfterSeconds float64         `json:"retryAfterSeconds"`
	ResourceType      string          `json:"resourceType"`
	ResourceID        string          `json:"resourceId"`
	FieldList         json.RawMessage `json:"fieldList"`
}

func newHTTPErrorDecoder(clock socialhub.Clock) func(int, http.Header, []byte) error {
	return func(status int, header http.Header, body []byte) error {
		var provider httpErrorPayload
		_ = json.Unmarshal(body, &provider)
		code, class := classifyAmazonError(status, provider)
		retryAfter := parseRetryAfter(header.Get("Retry-After"), clock.Now())
		if retryAfter == 0 && provider.RetryAfterSeconds > 0 && provider.RetryAfterSeconds <= float64((24*time.Hour)/time.Second) {
			retryAfter = time.Duration(provider.RetryAfterSeconds * float64(time.Second))
		}
		result := &socialhub.Error{
			Code: code, Class: class, Platform: platformName, Product: productName, Op: "http",
			HTTPStatus:      status,
			PlatformCode:    boundedMessage(joinPlatformCode(provider.Type, provider.Reason, provider.ResourceType), 256),
			PlatformMessage: "Amazon Creators API rejected the request",
			RequestID:       boundedMessage(firstHeader(header, "x-amzn-RequestId", "x-amzn-requestid", "X-Request-ID"), 256),
			RetryAfter:      retryAfter,
		}
		if code == socialhub.CodeApprovalRequired || code == socialhub.CodePermissionDenied {
			result.RequiredScopes = []string{oauthScope}
			result.ApprovalURL = documentationURL
		}
		return result
	}
}

func classifyAmazonError(status int, provider httpErrorPayload) (socialhub.ErrorCode, socialhub.ErrorClass) {
	if status == http.StatusForbidden && strings.EqualFold(strings.TrimSpace(provider.Reason), "AssociateNotEligible") {
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	}
	switch strings.ToLower(strings.TrimSpace(provider.Type)) {
	case "validationexception":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case "unauthorizedexception":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "throttleexception":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "resourcenotfoundexception":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "internalserverexception":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	}
	return classifyHTTPError(status)
}

func classifyHTTPError(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
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

func joinPlatformCode(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "/")
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

func redactSensitive(value string) string {
	for _, key := range []string{"access_token", "authorization", "client_secret", "credential_secret"} {
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

func redactExact(value, credential string) string {
	if credential == "" {
		return value
	}
	return strings.ReplaceAll(value, credential, "[REDACTED]")
}

func sanitizeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

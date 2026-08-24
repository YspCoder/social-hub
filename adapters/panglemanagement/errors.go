package panglemanagement

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

// ErrOutcomeUnknown means a mutation may have reached Pangle and must be reconciled before retrying.
var ErrOutcomeUnknown = errors.New("panglemanagement: mutation outcome unknown")

func businessError(operation string, status int, header http.Header, platformCode, envelopeRequestID string, now time.Time) error {
	code, class, message := classifyBusinessCode(platformCode)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode, PlatformMessage: message,
		RequestID:  firstNonEmptyBounded(envelopeRequestID, header.Get("X-Request-ID"), header.Get("X-Tt-Logid")),
		RetryAfter: parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func classifyBusinessCode(platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass, string) {
	switch platformCode {
	case "40001", "40003", "40005":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "Pangle rejected the Management API request parameters"
	case "41001":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the Management API signature"
	case "41003", "41005":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the configured publisher account or role"
	case "43001", "43003":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, "The configured Pangle role does not have permission for this operation"
	case "43005":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction, "The Pangle account is not allowlisted for this operation"
	case "45001", "120":
		return socialhub.CodeNotFound, socialhub.ClassPermanent, "The requested Pangle app or ad placement was not found"
	case "45003", "45005", "47007", "47008":
		return socialhub.CodeConflict, socialhub.ClassPermanent, "The requested Pangle status transition is not allowed"
	case "45007", "45009":
		return socialhub.CodeInvalidArgument, socialhub.ClassUserAction, "Pangle could not validate the submitted app information"
	case "47001", "47003":
		return socialhub.CodeConflict, socialhub.ClassRetryable, "A previous Pangle resource update is still in progress"
	case "47005", "121":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable, "The Pangle operation is rate-limited or still in its update cooldown"
	case "47006":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, "The Pangle account cannot edit CPM or has exceeded its edit allowance"
	case "50001", "50003", "50005", "116", "9998":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "Pangle Management API is temporarily unavailable"
	case "50007":
		return socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, "Pangle app verification is still processing asynchronously"
	case "101":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the expected-CPM signature"
	case "107":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction, "Pangle rejected the expected-CPM account"
	case "115", "122", "135", "136":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent, "Pangle rejected the expected-CPM parameters or currency"
	case "117":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction, "The configured Pangle role cannot access the requested app"
	case "123", "129":
		return socialhub.CodeUnsupported, socialhub.ClassPermanent, "The requested Pangle ad placement does not support expected CPM"
	default:
		return socialhub.CodePlatformError, socialhub.ClassPermanent, "Pangle rejected the Management API request"
	}
}

func httpStatusError(operation string, status int, header http.Header, platformCode, envelopeRequestID string, now time.Time) error {
	code, class := classifyHTTPStatus(status)
	result := &socialhub.Error{
		Code: code, Class: class, Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformCode: platformCode,
		PlatformMessage: "Pangle rejected the Management API HTTP request",
		RequestID:       firstNonEmptyBounded(envelopeRequestID, header.Get("X-Request-ID"), header.Get("X-Tt-Logid")),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After"), now),
	}
	setApprovalURL(result)
	return result
}

func setApprovalURL(err *socialhub.Error) {
	if err != nil && (err.Code == socialhub.CodeUnauthenticated || err.Code == socialhub.CodePermissionDenied ||
		err.Code == socialhub.CodeApprovalRequired) {
		err.ApprovalURL = documentationURL
	}
}

func classifyHTTPStatus(status int) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	case http.StatusUnauthorized:
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusPaymentRequired:
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
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
		Code: code, Class: class, Platform: platformName, Product: productName,
		Op: operation, Cause: sanitizeTransportError(cause),
	}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation, PlatformMessage: message,
	}
}

func authenticationError(operation, message string, cause error, secrets ...string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnauthenticated, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: message, ApprovalURL: documentationURL,
		Cause: sanitizeCredentialCause(cause, secrets...),
	}
}

func platformContractError(operation, message string, status int) error {
	return &socialhub.Error{
		Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
		Platform: platformName, Product: productName, Op: operation,
		HTTPStatus: status, PlatformMessage: message,
	}
}

func outcomeUnknownError(operation string, cause error) error {
	requestID := ""
	var hub *socialhub.Error
	if errors.As(cause, &hub) {
		requestID = hub.RequestID
	}
	return &socialhub.Error{
		Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
		Platform: platformName, Product: productName, Op: operation,
		PlatformMessage: "Pangle mutation outcome is unknown; query the resource state before retrying",
		RequestID:       requestID, Cause: errors.Join(ErrOutcomeUnknown, sanitizeTransportError(cause)),
	}
}

func withMutationOutcome(operation string, mutation bool, status int, err error) error {
	if err == nil || !mutation {
		return err
	}
	if status == 0 || status == http.StatusRequestTimeout || status >= 500 || status >= 200 && status < 300 {
		return outcomeUnknownError(operation, err)
	}
	return err
}

func sanitizeTransportError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = boundedHeader(value, 128)
	if value == "" {
		return 0
	}
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

func firstNonEmptyBounded(values ...string) string {
	for _, value := range values {
		if bounded := boundedHeader(value, 256); bounded != "" {
			return bounded
		}
	}
	return ""
}

func boundedHeader(value string, maximum int) string {
	if !utf8.ValidString(value) || len(value) > maximum || strings.ContainsFunc(value, unicode.IsControl) {
		return ""
	}
	return strings.TrimSpace(value)
}

func safeRequestID(value string, redactions ...string) string {
	value = boundedHeader(value, 256)
	if value == "" || containsAny(value, redactions...) {
		return ""
	}
	return value
}

func sanitizedResponseHeaders(header http.Header, redactions ...string) http.Header {
	result := make(http.Header)
	for _, name := range []string{"X-Request-ID", "X-Tt-Logid"} {
		if value := safeRequestID(header.Get(name), redactions...); value != "" {
			result.Set(name, value)
		}
	}
	if value := boundedHeader(header.Get("Retry-After"), 128); value != "" {
		result.Set("Retry-After", value)
	}
	return result
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func sanitizeCredentialCause(cause error, secrets ...string) error {
	if cause == nil {
		return nil
	}
	message := sanitizeTransportError(cause).Error()
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	message = redactCredentialValues(message)
	if !utf8.ValidString(message) || len(message) > 1024 || strings.ContainsFunc(message, unicode.IsControl) {
		message = "credential resolution failed"
	}
	return errors.New(message)
}

func redactCredentialValues(value string) string {
	markers := []string{
		"authorization", "security key", "security_key", "secret key", "secret_key", "signature", "sign", "token",
	}
	for _, marker := range markers {
		for cursor := 0; cursor < len(value); {
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
			for end < len(value) && !strings.ContainsRune("\r\n,;}&\"' \t", rune(value[end])) {
				end++
			}
			value = value[:start] + "[REDACTED]" + value[end:]
			cursor = start + len("[REDACTED]")
		}
	}
	return value
}

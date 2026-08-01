package dailymotion

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type errorEnvelope struct {
	Error struct {
		Code             string          `json:"code"`
		Message          string          `json:"message"`
		CorrelationID    string          `json:"correlation_id"`
		Details          json.RawMessage `json:"details"`
		DocumentationURL string          `json:"documentation_url"`
	} `json:"error"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response errorEnvelope
	_ = json.Unmarshal(body, &response)
	code, class := classifyError(status, response.Error.Code)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "dailymotion", Product: productName, Op: "http",
		HTTPStatus: status, PlatformCode: boundedMessage(response.Error.Code, 128),
		PlatformMessage: boundedMessage(response.Error.Message, 512),
		RequestID:       boundedMessage(firstNonEmpty(response.Error.CorrelationID, header.Get("X-Request-ID"), header.Get("X-Correlation-ID")), 512),
		RetryAfter:      parseRetryAfter(header.Get("Retry-After")),
	}
	if code == socialhub.CodeApprovalRequired {
		err.ApprovalURL = "https://developers.dailymotion.com/reference/api-scopes"
		var details struct {
			MissingPermissions  []string `json:"missing_permissions"`
			RequiredPermissions []string `json:"required_permissions"`
		}
		if json.Unmarshal(response.Error.Details, &details) == nil {
			err.RequiredScopes = details.MissingPermissions
			if len(err.RequiredScopes) == 0 {
				err.RequiredScopes = details.RequiredPermissions
			}
		}
	}
	return err
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "INVALID_AUTHORIZATION", "AUTHORIZATION_EXPIRED", "INVALID_TOKEN":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "MISSING_PERMISSIONS", "UPSTREAM_ACCESS_DENIED":
		return socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "RESOURCE_NOT_FOUND":
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	case "TOO_MANY_REQUESTS", "RATE_LIMIT_EXCEEDED":
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
		return socialhub.CodePlatformError, socialhub.ClassPermanent
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "dailymotion", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "dailymotion", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "dailymotion", Product: productName, Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

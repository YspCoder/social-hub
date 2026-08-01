package tiktok

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	LogID   string `json:"log_id"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope struct {
		Error            apiError `json:"error"`
		ErrorCode        string   `json:"error_code"`
		ErrorDescription string   `json:"error_description"`
		LogID            string   `json:"log_id"`
	}
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error.Code == "" {
		envelope.Error = apiError{Code: firstNonEmpty(envelope.ErrorCode, http.StatusText(status)), Message: envelope.ErrorDescription, LogID: envelope.LogID}
	}
	return mapAPIError(status, header, envelope.Error)
}

func checkAPIError(operation string, value apiError) error {
	if value.Code == "" || value.Code == "ok" {
		return nil
	}
	err := mapAPIError(http.StatusOK, nil, value)
	if typed, ok := err.(*socialhub.Error); ok {
		typed.Op = operation
	}
	return err
}

func mapAPIError(status int, header http.Header, value apiError) error {
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch value.Code {
	case "access_token_invalid", "invalid_token":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "scope_not_authorized":
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "invalid_param", "invalid_params", "invalid_request":
		code = socialhub.CodeInvalidArgument
	case "rate_limit_exceeded", "spam_risk_too_many_posts", "spam_risk_too_many_pending_share", "reached_active_user_cap":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "url_ownership_unverified", "privacy_level_option_mismatch", "unaudited_client_can_only_post_to_private_accounts", "spam_risk_user_banned_from_posting":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "internal_error":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	default:
		switch status {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			code = socialhub.CodeInvalidArgument
		case http.StatusUnauthorized:
			code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
		case http.StatusForbidden:
			code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
		case http.StatusNotFound:
			code = socialhub.CodeNotFound
		case http.StatusConflict:
			code = socialhub.CodeConflict
		case http.StatusTooManyRequests:
			code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
		default:
			if status >= 500 {
				code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
			}
		}
	}
	return &socialhub.Error{
		Code: code, Class: class, Platform: "tiktok", Product: "tiktok-for-developers", HTTPStatus: status,
		PlatformCode: value.Code, PlatformMessage: boundedMessage(value.Message, 512), RequestID: value.LogID,
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "tiktok", Product: "tiktok-for-developers", Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "tiktok", Product: "tiktok-for-developers", Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "tiktok", Product: "tiktok-for-developers", Op: operation, PlatformMessage: message}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

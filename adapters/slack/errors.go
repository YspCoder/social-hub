package slack

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func apiResponseError(operation string, response apiEnvelope) error {
	platformCode := strings.TrimSpace(response.Error)
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch platformCode {
	case "not_authed", "invalid_auth", "token_revoked", "token_expired", "account_inactive":
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "missing_scope":
		code, class = socialhub.CodeApprovalRequired, socialhub.ClassUserAction
	case "no_permission", "not_allowed_token_type", "restricted_action", "access_denied", "accesslimited", "ekm_access_denied", "enterprise_is_restricted", "team_access_not_granted", "not_in_channel", "no_access", "is_archived", "cant_update_message", "cant_delete_message":
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "channel_not_found", "user_not_found", "message_not_found", "file_not_found", "thread_not_found":
		code = socialhub.CodeNotFound
	case "already_reacted", "already_archived", "already_complete":
		code = socialhub.CodeConflict
	case "rate_limited", "ratelimited":
		code, class = socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "internal_error", "fatal_error", "request_timeout", "service_unavailable", "org_login_required", "team_added_to_org":
		code, class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
	case "method_deprecated", "deprecated_endpoint":
		code = socialhub.CodeUnsupported
	case "invalid_arguments", "invalid_arg_name", "invalid_array_arg", "invalid_blocks", "invalid_charset", "invalid_cursor", "invalid_form_data", "invalid_post_type", "missing_argument", "no_item_specified", "msg_too_long", "no_text", "no_reaction", "bad_timestamp":
		code = socialhub.CodeInvalidArgument
	}
	messageParts := make([]string, 0, 1+len(response.Metadata.Messages))
	if response.Warning != "" {
		messageParts = append(messageParts, response.Warning)
	}
	messageParts = append(messageParts, response.Metadata.Messages...)
	err := &socialhub.Error{
		Code: code, Class: class, Platform: "slack", Product: productName, Op: operation,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(strings.Join(messageParts, ": "), 512),
	}
	if code == socialhub.CodeApprovalRequired {
		err.RequiredScopes = splitScopes(response.Needed)
		err.ApprovalURL = "https://api.slack.com/apps"
	}
	return err
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var envelope apiEnvelope
	if json.Unmarshal(body, &envelope) == nil && strings.TrimSpace(envelope.Error) != "" {
		err := apiResponseError("http", envelope)
		if platformErr, ok := err.(*socialhub.Error); ok {
			platformErr.HTTPStatus = status
			platformErr.RequestID = slackRequestID(header)
			platformErr.RetryAfter = retryAfter(header.Get("Retry-After"))
			if status == http.StatusTooManyRequests {
				platformErr.Code, platformErr.Class = socialhub.CodeRateLimited, socialhub.ClassRetryable
			} else if status >= 500 {
				platformErr.Code, platformErr.Class = socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable
			}
		}
		return err
	}
	code, class := socialhub.CodePlatformError, socialhub.ClassPermanent
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		code = socialhub.CodeInvalidArgument
	case http.StatusUnauthorized:
		code, class = socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case http.StatusForbidden:
		code, class = socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case http.StatusNotFound, http.StatusGone:
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
	return &socialhub.Error{
		Code: code, Class: class, Platform: "slack", Product: productName, HTTPStatus: status,
		RequestID: slackRequestID(header), RetryAfter: retryAfter(header.Get("Retry-After")),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{Code: code, Class: class, Platform: "slack", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "slack", Product: productName, Op: operation, PlatformMessage: message}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "slack", Product: productName, Op: operation, PlatformMessage: message}
}

func slackRequestID(header http.Header) string {
	for _, name := range []string{"X-Slack-Req-Id", "X-Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func retryAfter(value string) time.Duration {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((24*time.Hour)/time.Second) {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func splitScopes(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func boundedMessage(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

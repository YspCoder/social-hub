package lemmy

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
	Error   string `json:"error"`
	Message string `json:"message"`
	Msg     string `json:"msg"`
}

func decodeHTTPError(status int, header http.Header, body []byte) error {
	var response apiErrorEnvelope
	_ = json.Unmarshal(body, &response)
	platformCode := boundedMessage(firstNonEmpty(response.Error, response.Msg), 256)
	code, class := classifyError(status, platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Platform: "lemmy", Product: productName, HTTPStatus: status,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(firstNonEmpty(response.Message, response.Error, response.Msg), 512),
		RequestID:  boundedMessage(firstNonEmpty(header.Get("X-Request-Id"), header.Get("X-Correlation-Id")), 256),
		RetryAfter: parseRetryAfter(header.Get("Retry-After")),
	}
}

func classifyError(status int, platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "rate_limit_error":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "not_logged_in", "incorrect_login", "invalid_password", "token_not_found":
		return socialhub.CodeUnauthenticated, socialhub.ClassUserAction
	case "not_a_moderator", "not_an_admin", "not_a_mod_or_admin", "site_ban", "banned", "banned_from_community",
		"person_is_banned_from_community", "person_is_banned_from_site", "only_mods_can_post_in_community", "post_is_locked", "locked":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "community_already_exists", "email_already_exists", "user_already_exists", "community_moderator_already_exists",
		"community_user_already_banned", "community_block_already_exists", "community_follower_already_exists",
		"person_block_already_exists", "instance_block_already_exists", "site_already_exists":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	}
	if strings.HasPrefix(platformCode, "couldnt_find_") {
		return socialhub.CodeNotFound, socialhub.ClassPermanent
	}
	if strings.HasPrefix(platformCode, "invalid_") || strings.HasSuffix(platformCode, "_too_long") ||
		platformCode == "no_id_given" || platformCode == "too_many_items" || platformCode == "blocked_url" || platformCode == "slurs" {
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	}
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
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
	return &socialhub.Error{Code: code, Class: class, Platform: "lemmy", Product: productName, Op: operation, Cause: cause}
}

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "lemmy", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "lemmy", Product: productName,
		Op: operation, PlatformMessage: message,
	}
}

func parseRetryAfter(value string) time.Duration {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

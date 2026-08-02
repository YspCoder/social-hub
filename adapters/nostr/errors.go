package nostr

import (
	"errors"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

var errInvalidIdentifier = errors.New("invalid Nostr identifier")

func invalidArgument(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent,
		Op: operation, Platform: "nostr", Product: productName, PlatformMessage: boundedMessage(message, 512),
	}
}

func unsupported(operation, message string) error {
	return &socialhub.Error{
		Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent,
		Op: operation, Platform: "nostr", Product: productName, PlatformMessage: boundedMessage(message, 512),
	}
}

func platformError(operation string, code socialhub.ErrorCode, class socialhub.ErrorClass, cause error) error {
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: "nostr", Product: productName, Cause: cause,
	}
}

func relayError(operation, reason string, cause error) error {
	platformCode, message := relayReason(reason)
	code, class := classifyRelayCode(platformCode)
	return &socialhub.Error{
		Code: code, Class: class, Op: operation, Platform: "nostr", Product: productName,
		PlatformCode: platformCode, PlatformMessage: boundedMessage(message, 512), Cause: cause,
	}
}

func relayReason(value string) (string, string) {
	message := strings.TrimSpace(value)
	message = strings.TrimSpace(strings.TrimPrefix(message, "msg:"))
	prefix, _, found := strings.Cut(message, ":")
	if !found {
		return "error", message
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	switch prefix {
	case "duplicate", "pow", "blocked", "rate-limited", "invalid", "restricted", "mute", "error":
		return prefix, message
	default:
		return "error", message
	}
}

func classifyRelayCode(platformCode string) (socialhub.ErrorCode, socialhub.ErrorClass) {
	switch platformCode {
	case "duplicate":
		return socialhub.CodeConflict, socialhub.ClassPermanent
	case "rate-limited":
		return socialhub.CodeRateLimited, socialhub.ClassRetryable
	case "blocked", "restricted", "mute":
		return socialhub.CodePermissionDenied, socialhub.ClassUserAction
	case "invalid", "pow":
		return socialhub.CodeInvalidArgument, socialhub.ClassPermanent
	default:
		return socialhub.CodePlatformError, socialhub.ClassRetryable
	}
}

func boundedMessage(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

package socialhub

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrorCode is a platform-neutral failure category.
type ErrorCode string

// ErrorClass indicates the action a caller should take.
type ErrorClass string

const (
	CodeInvalidArgument        ErrorCode = "invalid_argument"
	CodeUnauthenticated        ErrorCode = "unauthenticated"
	CodePermissionDenied       ErrorCode = "permission_denied"
	CodeApprovalRequired       ErrorCode = "approval_required"
	CodeUnsupported            ErrorCode = "unsupported"
	CodeNotFound               ErrorCode = "not_found"
	CodeConflict               ErrorCode = "conflict"
	CodeRateLimited            ErrorCode = "rate_limited"
	CodeTemporarilyUnavailable ErrorCode = "temporarily_unavailable"
	CodePlatformError          ErrorCode = "platform_error"
)

const (
	ClassUserAction ErrorClass = "user_action"
	ClassPermanent  ErrorClass = "permanent"
	ClassRetryable  ErrorClass = "retryable"
)

var (
	ErrAdapterNotFound  = errors.New("socialhub: adapter not found")
	ErrInvalidArgument  = errors.New("socialhub: invalid argument")
	ErrUnauthenticated  = errors.New("socialhub: unauthenticated")
	ErrPermissionDenied = errors.New("socialhub: permission denied")
	ErrApprovalRequired = errors.New("socialhub: approval required")
	ErrUnsupported      = errors.New("socialhub: unsupported capability")
	ErrNotFound         = errors.New("socialhub: not found")
	ErrConflict         = errors.New("socialhub: conflict")
	ErrRateLimited      = errors.New("socialhub: rate limited")
	ErrUnavailable      = errors.New("socialhub: temporarily unavailable")
)

// Error preserves a sanitized platform failure while exposing common behavior.
type Error struct {
	Code            ErrorCode
	Class           ErrorClass
	Op              string
	Platform        string
	Product         string
	AccountHash     string
	HTTPStatus      int
	PlatformCode    string
	PlatformMessage string
	RequestID       string
	RetryAfter      time.Duration
	RequiredScopes  []string
	ApprovalURL     string
	Cause           error
}

// Error returns a deliberately bounded message and never includes request or
// response bodies, credentials, or account identifiers.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	parts := []string{"socialhub"}
	if e.Platform != "" {
		parts = append(parts, e.Platform)
	}
	if e.Op != "" {
		parts = append(parts, e.Op)
	}
	if e.Code != "" {
		parts = append(parts, string(e.Code))
	}
	if e.PlatformCode != "" {
		parts = append(parts, "platform_code="+e.PlatformCode)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	return strings.Join(parts, ": ")
}

// Unwrap exposes the underlying transport or decoding failure.
func (e *Error) Unwrap() error { return e.Cause }

// Is makes common categories usable with errors.Is.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == sentinelForCode(e.Code) || errors.Is(e.Cause, target)
}

// Retryable reports whether retry policy may consider the failure transient.
func (e *Error) Retryable() bool { return e != nil && e.Class == ClassRetryable }

func sentinelForCode(code ErrorCode) error {
	switch code {
	case CodeInvalidArgument:
		return ErrInvalidArgument
	case CodeUnauthenticated:
		return ErrUnauthenticated
	case CodePermissionDenied:
		return ErrPermissionDenied
	case CodeApprovalRequired:
		return ErrApprovalRequired
	case CodeUnsupported:
		return ErrUnsupported
	case CodeNotFound:
		return ErrNotFound
	case CodeConflict:
		return ErrConflict
	case CodeRateLimited:
		return ErrRateLimited
	case CodeTemporarilyUnavailable:
		return ErrUnavailable
	default:
		return nil
	}
}

// UnsupportedError describes a capability that an adapter cannot expose.
func UnsupportedError(platform Platform, capability Capability) error {
	return &Error{
		Code:            CodeUnsupported,
		Class:           ClassPermanent,
		Platform:        string(platform),
		Op:              string(capability),
		PlatformMessage: fmt.Sprintf("%s is not supported", capability),
	}
}

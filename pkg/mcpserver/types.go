package mcpserver

import (
	"context"
	"errors"
	"time"

	"social-hub/pkg/socialhub"
)

const maxCallTimeout = 2 * time.Minute

// TargetRef selects one configured account without accepting credentials.
type TargetRef struct {
	Adapter   string `json:"adapter" jsonschema:"Registered social-hub adapter name"`
	AccountID string `json:"account_id" jsonschema:"Configured account identifier"`
}

// CallControl contains bounded per-call metadata.
type CallControl struct {
	RequestID      string   `json:"request_id,omitempty" jsonschema:"Caller-generated request identifier"`
	IdempotencyKey string   `json:"idempotency_key,omitempty" jsonschema:"Idempotency key for adapters that support it"`
	TimeoutMS      int      `json:"timeout_ms,omitempty" jsonschema:"Call timeout in milliseconds, from 0 through 120000"`
	Fields         []string `json:"fields,omitempty" jsonschema:"Optional platform fields requested from adapters that support field selection"`
}

// TargetInfo is the non-secret deployment metadata exposed to agents.
type TargetInfo struct {
	Adapter    string `json:"adapter"`
	AccountID  string `json:"account_id"`
	Product    string `json:"product,omitempty"`
	APIVersion string `json:"api_version,omitempty"`
	DocURL     string `json:"doc_url,omitempty"`
}

// CapabilityInfo is the stable, ordered form of a capability declaration.
type CapabilityInfo struct {
	Name      socialhub.Capability    `json:"name"`
	Supported bool                    `json:"supported"`
	Approval  socialhub.ApprovalState `json:"approval"`
	Scopes    []string                `json:"scopes,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	DocURL    string                  `json:"doc_url,omitempty"`
}

// ToolError deliberately excludes platform response bodies, credential
// references, account hashes, and raw provider messages.
type ToolError struct {
	Code           socialhub.ErrorCode  `json:"code"`
	Class          socialhub.ErrorClass `json:"class"`
	Op             string               `json:"op,omitempty"`
	Platform       string               `json:"platform,omitempty"`
	Product        string               `json:"product,omitempty"`
	HTTPStatus     int                  `json:"http_status,omitempty"`
	RequestID      string               `json:"request_id,omitempty"`
	RetryAfterMS   int64                `json:"retry_after_ms,omitempty"`
	RequiredScopes []string             `json:"required_scopes,omitempty"`
	ApprovalURL    string               `json:"approval_url,omitempty"`
}

// Result is the structured output envelope used by every tool.
type Result[T any] struct {
	Data  *T         `json:"data,omitempty"`
	Error *ToolError `json:"error,omitempty"`
}

type TargetsData struct {
	Targets []TargetInfo `json:"targets"`
}

type CapabilitiesData struct {
	Target       TargetRef          `json:"target"`
	Platform     socialhub.Platform `json:"platform"`
	Capabilities []CapabilityInfo   `json:"capabilities"`
}

type UserData struct {
	User *socialhub.User `json:"user"`
}

type PostData struct {
	Post *socialhub.Post `json:"post"`
}

type PostsData struct {
	Page socialhub.Page[socialhub.Post] `json:"page"`
}

type CommentsData struct {
	Page socialhub.Page[socialhub.Comment] `json:"page"`
}

type CommentData struct {
	Comment *socialhub.Comment `json:"comment"`
}

type MessageData struct {
	Message *socialhub.Message `json:"message"`
}

type PublishStatusData struct {
	Status *socialhub.PublishStatus `json:"status"`
}

type MutationData struct {
	Success bool `json:"success"`
}

func sanitizeError(err error) *ToolError {
	var platformError *socialhub.Error
	if errors.As(err, &platformError) {
		return &ToolError{
			Code:           platformError.Code,
			Class:          platformError.Class,
			Op:             platformError.Op,
			Platform:       platformError.Platform,
			Product:        platformError.Product,
			HTTPStatus:     platformError.HTTPStatus,
			RequestID:      platformError.RequestID,
			RetryAfterMS:   platformError.RetryAfter.Milliseconds(),
			RequiredScopes: append([]string(nil), platformError.RequiredScopes...),
			ApprovalURL:    platformError.ApprovalURL,
		}
	}
	return &ToolError{Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent, Op: "mcp_tool"}
}

func callContext(ctx context.Context, control CallControl) (context.Context, context.CancelFunc, []socialhub.CallOption, error) {
	if control.TimeoutMS < 0 || control.TimeoutMS > int(maxCallTimeout/time.Millisecond) {
		return nil, nil, nil, invalidArgument("call_control", "timeout_ms must be between 0 and 120000")
	}
	options := make([]socialhub.CallOption, 0, 4)
	if control.RequestID != "" {
		options = append(options, socialhub.WithRequestID(control.RequestID))
	}
	if control.IdempotencyKey != "" {
		options = append(options, socialhub.WithIdempotencyKey(control.IdempotencyKey))
	}
	if len(control.Fields) > 0 {
		options = append(options, socialhub.WithFields(control.Fields...))
	}
	if control.TimeoutMS == 0 {
		return ctx, func() {}, options, nil
	}
	timeout := time.Duration(control.TimeoutMS) * time.Millisecond
	options = append(options, socialhub.WithCallTimeout(timeout))
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	return callCtx, cancel, options, nil
}

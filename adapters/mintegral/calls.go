package mintegral

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func callContext(ctx context.Context, operation string, options []socialhub.CallOption) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, invalidArgument(operation, "context must not be nil")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" || resolved.IdempotencyKey != "" || len(resolved.Fields) > 0 {
		return nil, nil, invalidArgument(operation, "request IDs, idempotency keys, and field selection are not supported")
	}
	if resolved.Timeout > 0 {
		callCtx, cancel := context.WithTimeout(ctx, resolved.Timeout)
		return callCtx, cancel, nil
	}
	return ctx, func() {}, nil
}

func callValue[T any](ctx context.Context, operation string, options []socialhub.CallOption, invoke func(context.Context) (T, error)) (T, error) {
	var zero T
	callCtx, cancel, err := callContext(ctx, operation, options)
	if err != nil {
		return zero, err
	}
	defer cancel()
	result, err := invoke(callCtx)
	if err != nil {
		return zero, mapError(operation, err)
	}
	return result, nil
}

func callNoValue(ctx context.Context, operation string, options []socialhub.CallOption, invoke func(context.Context) error) error {
	callCtx, cancel, err := callContext(ctx, operation, options)
	if err != nil {
		return err
	}
	defer cancel()
	return mapError(operation, invoke(callCtx))
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

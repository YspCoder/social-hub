package socialhub

import "time"

// CallOptions controls one API operation.
type CallOptions struct {
	RequestID      string
	IdempotencyKey string
	Timeout        time.Duration
	Fields         []string
}

// CallOption configures one API operation.
type CallOption func(*CallOptions) error

// ResolveCallOptions validates and combines call options.
func ResolveCallOptions(options ...CallOption) (CallOptions, error) {
	var resolved CallOptions
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&resolved); err != nil {
			return CallOptions{}, err
		}
	}
	return resolved, nil
}

// WithRequestID associates a caller-generated request identifier with a call.
func WithRequestID(requestID string) CallOption {
	return func(options *CallOptions) error {
		options.RequestID = requestID
		return nil
	}
}

// WithIdempotencyKey supplies an idempotency key to adapters that support it.
func WithIdempotencyKey(key string) CallOption {
	return func(options *CallOptions) error {
		options.IdempotencyKey = key
		return nil
	}
}

// WithCallTimeout overrides the client default timeout for one operation.
func WithCallTimeout(timeout time.Duration) CallOption {
	return func(options *CallOptions) error {
		if timeout < 0 {
			return &Error{Code: CodeInvalidArgument, Class: ClassPermanent, Op: "call_options", PlatformMessage: "timeout must not be negative"}
		}
		options.Timeout = timeout
		return nil
	}
}

// WithFields requests optional fields from adapters that support field selection.
func WithFields(fields ...string) CallOption {
	return func(options *CallOptions) error {
		options.Fields = append([]string(nil), fields...)
		return nil
	}
}

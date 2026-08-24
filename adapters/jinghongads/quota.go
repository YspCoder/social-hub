package jinghongads

import "time"

// QuotaPolicy records the authorized-account request limits published for the
// mainland Jinghong Marketing API. It is metadata for a shared limiter; the
// adapter does not keep an in-process counter.
type QuotaPolicy struct {
	MinuteRequests int
	MinuteWindow   time.Duration
	DailyRequests  int
	DailyWindow    time.Duration
	Scope          string
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		MinuteRequests: 600,
		MinuteWindow:   time.Minute,
		DailyRequests:  360_000,
		DailyWindow:    24 * time.Hour,
		Scope:          "authorized_account",
	}
}

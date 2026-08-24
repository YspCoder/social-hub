package applovinads

import "time"

// QuotaPolicy documents the account/key-scoped limits published by AppLovin.
// Enforcement belongs in social-hub's shared limiter so all processes using a
// key coordinate against the same budget.
type QuotaPolicy struct {
	Requests       int
	Window         time.Duration
	Penalty        time.Duration
	ErrorResponses int
	ErrorWindow    time.Duration
	ErrorPenalty   time.Duration
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		Requests: 1000, Window: time.Minute, Penalty: 10 * time.Minute,
		ErrorResponses: 100, ErrorWindow: 5 * time.Minute, ErrorPenalty: 24 * time.Hour,
	}
}

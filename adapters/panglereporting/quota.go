package panglereporting

import "time"

// QuotaPolicy records the Reporting API 2.0 request and response limits. It is
// metadata for a shared limiter; the adapter does not keep an in-process
// counter.
type QuotaPolicy struct {
	Requests         int
	Window           time.Duration
	MaximumRows      int
	MaximumClockSkew time.Duration
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		Requests: 2, Window: time.Second, MaximumRows: MaximumReportRows,
		MaximumClockSkew: 10 * time.Minute,
	}
}

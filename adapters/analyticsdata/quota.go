package analyticsdata

// QuotaPolicy records the published standard-property bounds. Analytics 360
// properties have higher token, concurrency, and server-error quotas.
type QuotaPolicy struct {
	CoreTokensPerPropertyPerDay           int
	CoreTokensPerPropertyPerHour          int
	CoreTokensPerProjectPropertyPerHour   int
	RealtimeTokensPerPropertyPerDay       int
	RealtimeTokensPerPropertyPerHour      int
	RealtimeTokensPerProjectPropertyHour  int
	ConcurrentRequestsPerProperty         int
	ServerErrorsPerProjectPropertyPerHour int
	PotentiallyThresholdedRequestsPerHour int
	MaximumRowsPerRequest                 int64
	MaximumBatchRequests                  int
	MaximumDateRanges                     int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		CoreTokensPerPropertyPerDay:           200_000,
		CoreTokensPerPropertyPerHour:          40_000,
		CoreTokensPerProjectPropertyPerHour:   14_000,
		RealtimeTokensPerPropertyPerDay:       200_000,
		RealtimeTokensPerPropertyPerHour:      40_000,
		RealtimeTokensPerProjectPropertyHour:  14_000,
		ConcurrentRequestsPerProperty:         10,
		ServerErrorsPerProjectPropertyPerHour: 10,
		PotentiallyThresholdedRequestsPerHour: 120,
		MaximumRowsPerRequest:                 250_000,
		MaximumBatchRequests:                  5,
		MaximumDateRanges:                     4,
	}
}

package admanager

import "time"

// QuotaPolicy records published Ad Manager network limits and v1 page bounds.
// Network and reporting-system overrides remain authoritative at runtime.
type QuotaPolicy struct {
	StandardNetworkRequestsPerSecond int
	AdManager360RequestsPerSecond    int
	InitialQuotaRetryDelay           time.Duration
	MaximumListPageSize              int
	MaximumReportRowsPageSize        int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		StandardNetworkRequestsPerSecond: 2,
		AdManager360RequestsPerSecond:    8,
		InitialQuotaRetryDelay:           5 * time.Second,
		MaximumListPageSize:              1000,
		MaximumReportRowsPageSize:        10000,
	}
}

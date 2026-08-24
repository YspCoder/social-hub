package adsense

// QuotaPolicy records the published AdSense Management API request and report
// bounds. Console-assigned and dynamic report-row quotas remain authoritative.
type QuotaPolicy struct {
	RequestsPerMinutePerUserProject int
	RequestsPerMinutePerProject     int
	RequestsPerDay                  int
	MaximumListPageSize             int32
	MaximumJSONReportRows           int32
	MaximumCSVReportRows            int32
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		RequestsPerMinutePerUserProject: 100,
		RequestsPerMinutePerProject:     500,
		RequestsPerDay:                  10_000,
		MaximumListPageSize:             10_000,
		MaximumJSONReportRows:           100_000,
		MaximumCSVReportRows:            1_000_000,
	}
}

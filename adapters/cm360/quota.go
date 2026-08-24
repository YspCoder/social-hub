package cm360

// QuotaPolicy records CM360's published default API and ReportData limits.
// Google Cloud projects and Report Builder accounts can have adjusted quotas.
type QuotaPolicy struct {
	ProjectRequestsPerDay          int
	ProjectQueriesPerSecond        int
	ProjectMaximumQueriesPerSecond int
	ReportDataRequestsPerMinute    int
	ReportDataRequestsPerDay       int
	ReportDataTimeoutSeconds       int
	RecommendedWriteConcurrency    int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		ProjectRequestsPerDay: 50000, ProjectQueriesPerSecond: 1, ProjectMaximumQueriesPerSecond: 10,
		ReportDataRequestsPerMinute: 120, ReportDataRequestsPerDay: 10000, ReportDataTimeoutSeconds: 60,
		RecommendedWriteConcurrency: 1,
	}
}

package admob

// QuotaPolicy records the published AdMob API request and report bounds.
// Additional back-end processing cost caps remain dynamic and authoritative.
type QuotaPolicy struct {
	AccountReadsPerMinutePerProject   int
	InventoryReadsPerMinutePerProject int
	InventoryReadsPerDayPerProject    int
	ReportingReadsPerMinutePerProject int
	DefaultInventoryPageSize          int32
	MaximumInventoryPageSize          int32
	MaximumReportRows                 int32
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		AccountReadsPerMinutePerProject: 900, InventoryReadsPerMinutePerProject: 120,
		InventoryReadsPerDayPerProject: 172_800, ReportingReadsPerMinutePerProject: 900,
		DefaultInventoryPageSize: 10_000, MaximumInventoryPageSize: 20_000,
		MaximumReportRows: 100_000,
	}
}

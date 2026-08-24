package unitypublisher

// QuotaPolicy documents Unity's current Publisher Manage API v2 limits.
// Enforcement belongs in social-hub's shared limiter so one process can
// coordinate all clients for the same credential and origin.
type QuotaPolicy struct {
	IPRequestsPerSecond       int
	ReadRequestsPerSecond     int
	ReadRequestsPerHour       int
	CreateRequestsPerSecond   int
	CreateRequestsPerHour     int
	MutationRequestsPerSecond int
	MutationRequestsPerHour   int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		IPRequestsPerSecond: 40, ReadRequestsPerSecond: 20, ReadRequestsPerHour: 8000,
		CreateRequestsPerSecond: 1, CreateRequestsPerHour: 60,
		MutationRequestsPerSecond: 1, MutationRequestsPerHour: 200,
	}
}

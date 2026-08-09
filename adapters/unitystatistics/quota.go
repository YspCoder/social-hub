package unitystatistics

// QuotaPolicy documents Unity's aggregate limits. Both dimensions are
// enforced independently and the first exhausted dimension wins.
type QuotaPolicy struct {
	RequestsPerSecond   int
	RequestsPer30Minute int
	Dimensions          []string
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		RequestsPerSecond: 1, RequestsPer30Minute: 30,
		Dimensions: []string{"organization_id", "ip_address"},
	}
}

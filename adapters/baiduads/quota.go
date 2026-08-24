package baiduads

// QuotaPolicy records Baidu's published per-interface QPS defaults. Account
// grants can differ, so callers should treat this as an initial limiter policy.
type QuotaPolicy struct {
	DefaultQPS        int
	CampaignGetQPS    int
	CampaignAddQPS    int
	CampaignUpdateQPS int
	CampaignDeleteQPS int
	AdGroupGetQPS     int
	AdGroupAddQPS     int
	AdGroupUpdateQPS  int
	AdGroupDeleteQPS  int
	CreativeGetQPS    int
	CreativeAddQPS    int
	CreativeUpdateQPS int
	CreativeDeleteQPS int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{
		DefaultQPS:     10,
		CampaignGetQPS: 100, CampaignAddQPS: 30, CampaignUpdateQPS: 30, CampaignDeleteQPS: 10,
		AdGroupGetQPS: 50, AdGroupAddQPS: 30, AdGroupUpdateQPS: 100, AdGroupDeleteQPS: 30,
		CreativeGetQPS: 100, CreativeAddQPS: 50, CreativeUpdateQPS: 100, CreativeDeleteQPS: 30,
	}
}

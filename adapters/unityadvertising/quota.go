package unityadvertising

// EndpointQuota documents a Unity endpoint group's aggregate limits.
type EndpointQuota struct {
	RequestsPerSecond   int
	RequestsPer30Minute int
}

// QuotaPolicy documents Unity's current Advertising Management API v1 limits.
// Enforcement belongs in social-hub's shared limiter so all clients using the
// same credential and origin coordinate their budgets.
type QuotaPolicy struct {
	Apps          EndpointQuota
	Bids          EndpointQuota
	Campaigns     EndpointQuota
	Creatives     EndpointQuota
	CreativePacks EndpointQuota
	Create        EndpointQuota
	Mutation      EndpointQuota
}

func DefaultQuotaPolicy() QuotaPolicy {
	standard := EndpointQuota{RequestsPerSecond: 20, RequestsPer30Minute: 4000}
	return QuotaPolicy{
		Apps: standard, Bids: EndpointQuota{RequestsPerSecond: 10, RequestsPer30Minute: 4000},
		Campaigns: standard, Creatives: standard, CreativePacks: standard,
		Create:   EndpointQuota{RequestsPerSecond: 1, RequestsPer30Minute: 30},
		Mutation: EndpointQuota{RequestsPerSecond: 1, RequestsPer30Minute: 100},
	}
}

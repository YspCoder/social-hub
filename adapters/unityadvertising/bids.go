package unityadvertising

import (
	"context"

	"social-hub/pkg/socialhub"
)

type CPIBid struct {
	Country CountryCode `json:"country"`
	Bid     BidAmount   `json:"bid"`
}

type CPIBidPatch struct {
	Country CountryCode `json:"country"`
	Bid     *BidAmount  `json:"bid"`
}

type SourceBid struct {
	Country     CountryCode `json:"country"`
	SourceAppID string      `json:"sourceAppId"`
	Bid         BidAmount   `json:"bid"`
}

type SourceBidPatch struct {
	Country     CountryCode `json:"country"`
	SourceAppID string      `json:"sourceAppId"`
	Bid         *BidAmount  `json:"bid"`
}

type ROASBid struct {
	Country CountryCode `json:"country"`
	Goal    ROASGoal    `json:"goal"`
	MaxBid  BidAmount   `json:"maxBid"`
}

type ROASBidReplace struct {
	Country CountryCode `json:"country"`
	Goal    ROASGoal    `json:"goal"`
	MaxBid  *BidAmount  `json:"maxBid,omitempty"`
}

type ROASBidPatch struct {
	Country CountryCode `json:"country"`
	Goal    *ROASGoal   `json:"goal"`
	MaxBid  *BidAmount  `json:"maxBid,omitempty"`
}

type RetentionBid struct {
	Country CountryCode `json:"country"`
	BaseBid BidAmount   `json:"baseBid"`
	MaxBid  BidAmount   `json:"maxBid"`
}

type RetentionBidPatch struct {
	Country CountryCode `json:"country"`
	BaseBid *BidAmount  `json:"baseBid"`
	MaxBid  BidAmount   `json:"maxBid"`
}

type EventOptimizationInfo struct {
	EventOptimizationType EventOptimizationType `json:"eventOptimizationType"`
	Countries             []CountryCode         `json:"country"`
}

type EventOptimizationBid struct {
	Country CountryCode `json:"country"`
	Bid     BidAmount   `json:"bid"`
}

type EventOptimizationBidPatch struct {
	Country CountryCode `json:"country"`
	Bid     *BidAmount  `json:"bid"`
}

type SDKEventNamesInfo struct {
	EventOptimizationType EventOptimizationType `json:"eventOptimizationType"`
	SDKEventNames         []SDKEventName        `json:"sdkEventNames"`
}

type BidsWorkflow interface {
	ListCPIBids(context.Context, string, string, ...socialhub.CallOption) (Page[CPIBid], error)
	ReplaceCPIBids(context.Context, string, string, []CPIBid, ...socialhub.CallOption) (Page[CPIBid], error)
	PatchCPIBids(context.Context, string, string, []CPIBidPatch, ...socialhub.CallOption) (Page[CPIBid], error)
	ListSourceBids(context.Context, string, string, ...socialhub.CallOption) (Page[SourceBid], error)
	ReplaceSourceBids(context.Context, string, string, []SourceBid, ...socialhub.CallOption) (Page[SourceBid], error)
	PatchSourceBids(context.Context, string, string, []SourceBidPatch, ...socialhub.CallOption) (Page[SourceBid], error)
	ListROASBids(context.Context, string, string, ...socialhub.CallOption) (Page[ROASBid], error)
	ReplaceROASBids(context.Context, string, string, []ROASBidReplace, ...socialhub.CallOption) (Page[ROASBid], error)
	PatchROASBids(context.Context, string, string, []ROASBidPatch, ...socialhub.CallOption) (Page[ROASBid], error)
	ListRetentionBids(context.Context, string, string, ...socialhub.CallOption) (Page[RetentionBid], error)
	ReplaceRetentionBids(context.Context, string, string, []RetentionBid, ...socialhub.CallOption) (Page[RetentionBid], error)
	PatchRetentionBids(context.Context, string, string, []RetentionBidPatch, ...socialhub.CallOption) (Page[RetentionBid], error)
	ListEventOptimizationInfo(context.Context, string, ...socialhub.CallOption) (Page[EventOptimizationInfo], error)
	ListEventOptimizationBids(context.Context, string, string, ...socialhub.CallOption) (Page[EventOptimizationBid], error)
	ReplaceEventOptimizationBids(context.Context, string, string, []EventOptimizationBid, ...socialhub.CallOption) (Page[EventOptimizationBid], error)
	PatchEventOptimizationBids(context.Context, string, string, []EventOptimizationBidPatch, ...socialhub.CallOption) (Page[EventOptimizationBid], error)
}

func (client *Client) ListCPIBids(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[CPIBid], error) {
	return listCampaignResource(client, ctx, "cpi_bid_list", campaignSetID, campaignID, "/cpi-bids", validCPIBid, options...)
}

func (client *Client) ReplaceCPIBids(ctx context.Context, campaignSetID, campaignID string, input []CPIBid, options ...socialhub.CallOption) (Page[CPIBid], error) {
	if !validCPIBids(input) {
		return Page[CPIBid]{}, invalidArgument("cpi_bid_replace", "CPI bids contain an invalid country, bid, or duplicate country")
	}
	return mutateCampaignResource(client, ctx, "cpi_bid_replace", campaignSetID, campaignID, "/cpi-bids", input, validCPIBid, false, options...)
}

func (client *Client) PatchCPIBids(ctx context.Context, campaignSetID, campaignID string, input []CPIBidPatch, options ...socialhub.CallOption) (Page[CPIBid], error) {
	if !validCPIBidPatches(input) {
		return Page[CPIBid]{}, invalidArgument("cpi_bid_patch", "CPI bid patches contain an invalid country, bid, or duplicate country")
	}
	return mutateCampaignResource(client, ctx, "cpi_bid_patch", campaignSetID, campaignID, "/cpi-bids", input, validCPIBid, true, options...)
}

func (client *Client) ListSourceBids(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[SourceBid], error) {
	return listCampaignResource(client, ctx, "source_bid_list", campaignSetID, campaignID, "/source-bids", validSourceBid, options...)
}

func (client *Client) ReplaceSourceBids(ctx context.Context, campaignSetID, campaignID string, input []SourceBid, options ...socialhub.CallOption) (Page[SourceBid], error) {
	if !validSourceBids(input) {
		return Page[SourceBid]{}, invalidArgument("source_bid_replace", "source bids contain an invalid key, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "source_bid_replace", campaignSetID, campaignID, "/source-bids", input, validSourceBid, false, options...)
}

func (client *Client) PatchSourceBids(ctx context.Context, campaignSetID, campaignID string, input []SourceBidPatch, options ...socialhub.CallOption) (Page[SourceBid], error) {
	if !validSourceBidPatches(input) {
		return Page[SourceBid]{}, invalidArgument("source_bid_patch", "source bid patches contain an invalid key, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "source_bid_patch", campaignSetID, campaignID, "/source-bids", input, validSourceBid, true, options...)
}

func (client *Client) ListROASBids(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[ROASBid], error) {
	return listCampaignResource(client, ctx, "roas_bid_list", campaignSetID, campaignID, "/roas-bids", validROASBid, options...)
}

func (client *Client) ReplaceROASBids(ctx context.Context, campaignSetID, campaignID string, input []ROASBidReplace, options ...socialhub.CallOption) (Page[ROASBid], error) {
	if !validROASBidReplacements(input) {
		return Page[ROASBid]{}, invalidArgument("roas_bid_replace", "ROAS bids contain an invalid country, goal, max bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "roas_bid_replace", campaignSetID, campaignID, "/roas-bids", input, validROASBid, false, options...)
}

func (client *Client) PatchROASBids(ctx context.Context, campaignSetID, campaignID string, input []ROASBidPatch, options ...socialhub.CallOption) (Page[ROASBid], error) {
	if !validROASBidPatches(input) {
		return Page[ROASBid]{}, invalidArgument("roas_bid_patch", "ROAS bid patches contain an invalid country, goal, max bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "roas_bid_patch", campaignSetID, campaignID, "/roas-bids", input, validROASBid, true, options...)
}

func (client *Client) ListRetentionBids(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[RetentionBid], error) {
	return listCampaignResource(client, ctx, "retention_bid_list", campaignSetID, campaignID, "/retention-bids", validRetentionBid, options...)
}

func (client *Client) ReplaceRetentionBids(ctx context.Context, campaignSetID, campaignID string, input []RetentionBid, options ...socialhub.CallOption) (Page[RetentionBid], error) {
	if !validRetentionBids(input, false) {
		return Page[RetentionBid]{}, invalidArgument("retention_bid_replace", "retention bids contain an invalid country, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "retention_bid_replace", campaignSetID, campaignID, "/retention-bids", input, validRetentionBid, false, options...)
}

func (client *Client) PatchRetentionBids(ctx context.Context, campaignSetID, campaignID string, input []RetentionBidPatch, options ...socialhub.CallOption) (Page[RetentionBid], error) {
	if !validRetentionBidPatches(input) {
		return Page[RetentionBid]{}, invalidArgument("retention_bid_patch", "retention bid patches contain an invalid country, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "retention_bid_patch", campaignSetID, campaignID, "/retention-bids", input, validRetentionBid, true, options...)
}

func (client *Client) ListEventOptimizationInfo(ctx context.Context, campaignSetID string, options ...socialhub.CallOption) (Page[EventOptimizationInfo], error) {
	appPath, err := client.appPath("event_optimization_info_list", campaignSetID)
	if err != nil {
		return Page[EventOptimizationInfo]{}, err
	}
	var page Page[EventOptimizationInfo]
	if err := client.getJSON(ctx, "event_optimization_info_list", appPath+"/audience-pinpointer/event-optimization-info", nil, &page, options...); err != nil {
		return Page[EventOptimizationInfo]{}, err
	}
	if !validPage(page, 0, validEventOptimizationInfo) {
		return Page[EventOptimizationInfo]{}, platformContractError("event_optimization_info_list", "Unity returned invalid event optimization information")
	}
	return page, nil
}

func (client *Client) ListEventOptimizationBids(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (Page[EventOptimizationBid], error) {
	return listCampaignResource(client, ctx, "event_optimization_bid_list", campaignSetID, campaignID, "/event-optimization-bids", validEventOptimizationBid, options...)
}

func (client *Client) ReplaceEventOptimizationBids(ctx context.Context, campaignSetID, campaignID string, input []EventOptimizationBid, options ...socialhub.CallOption) (Page[EventOptimizationBid], error) {
	if !validEventOptimizationBids(input) {
		return Page[EventOptimizationBid]{}, invalidArgument("event_optimization_bid_replace", "event optimization bids contain an invalid country, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "event_optimization_bid_replace", campaignSetID, campaignID, "/event-optimization-bids", input, validEventOptimizationBid, false, options...)
}

func (client *Client) PatchEventOptimizationBids(ctx context.Context, campaignSetID, campaignID string, input []EventOptimizationBidPatch, options ...socialhub.CallOption) (Page[EventOptimizationBid], error) {
	if !validEventOptimizationBidPatches(input) {
		return Page[EventOptimizationBid]{}, invalidArgument("event_optimization_bid_patch", "event optimization bid patches contain an invalid country, bid, or duplicate")
	}
	return mutateCampaignResource(client, ctx, "event_optimization_bid_patch", campaignSetID, campaignID, "/event-optimization-bids", input, validEventOptimizationBid, true, options...)
}

func (client *Client) ListSDKEventNames(ctx context.Context, campaignSetID string, options ...socialhub.CallOption) (Page[SDKEventNamesInfo], error) {
	appPath, err := client.appPath("sdk_event_name_list", campaignSetID)
	if err != nil {
		return Page[SDKEventNamesInfo]{}, err
	}
	var page Page[SDKEventNamesInfo]
	if err := client.getJSON(ctx, "sdk_event_name_list", appPath+"/audience-pinpointer/sdk-event-names", nil, &page, options...); err != nil {
		return Page[SDKEventNamesInfo]{}, err
	}
	if !validPage(page, 0, validSDKEventNamesInfo) {
		return Page[SDKEventNamesInfo]{}, platformContractError("sdk_event_name_list", "Unity returned invalid SDK event names")
	}
	return page, nil
}

func listCampaignResource[T any](client *Client, ctx context.Context, operation, campaignSetID, campaignID, suffix string, valid func(T) bool, options ...socialhub.CallOption) (Page[T], error) {
	path, err := client.campaignPath(operation, campaignSetID, campaignID)
	if err != nil {
		return Page[T]{}, err
	}
	var page Page[T]
	if err := client.getJSON(ctx, operation, path+suffix, nil, &page, options...); err != nil {
		return Page[T]{}, err
	}
	if !validPage(page, 0, valid) {
		return Page[T]{}, platformContractError(operation, "Unity returned an invalid bid page")
	}
	return page, nil
}

func mutateCampaignResource[Input, Output any](client *Client, ctx context.Context, operation, campaignSetID, campaignID, suffix string, input Input, valid func(Output) bool, patch bool, options ...socialhub.CallOption) (Page[Output], error) {
	path, err := client.campaignPath(operation, campaignSetID, campaignID)
	if err != nil {
		return Page[Output]{}, err
	}
	var page Page[Output]
	if patch {
		err = client.patchJSON(ctx, operation, path+suffix, input, &page, options...)
	} else {
		err = client.putJSON(ctx, operation, path+suffix, input, &page, options...)
	}
	if err != nil {
		return Page[Output]{}, err
	}
	if !validPage(page, 0, valid) {
		return Page[Output]{}, platformContractError(operation, "Unity returned an invalid bid page")
	}
	return page, nil
}

func validCPIBid(value CPIBid) bool { return validCountry(value.Country) && validBid(value.Bid) }

func validCPIBids(values []CPIBid) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCPIBid(value) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validCPIBidPatches(values []CPIBidPatch) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value.Country) || value.Bid != nil && !validBid(*value.Bid) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validSourceBid(value SourceBid) bool {
	return validCountry(value.Country) && validSourceAppID(value.SourceAppID) && validBid(value.Bid)
}

func validSourceBids(values []SourceBid) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := string(value.Country) + "\x00" + value.SourceAppID
		if !validSourceBid(value) || duplicateString(seen, key) {
			return false
		}
	}
	return true
}

func validSourceBidPatches(values []SourceBidPatch) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := string(value.Country) + "\x00" + value.SourceAppID
		if !validCountry(value.Country) || !validSourceAppID(value.SourceAppID) || value.Bid != nil && !validBid(*value.Bid) || duplicateString(seen, key) {
			return false
		}
	}
	return true
}

func validROASBid(value ROASBid) bool {
	return validCountry(value.Country) && validROASGoal(value.Goal) && validResponseMaxBid(value.MaxBid)
}

func validROASBidReplacements(values []ROASBidReplace) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value.Country) || !validROASGoal(value.Goal) || value.MaxBid != nil && !validBid(*value.MaxBid) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validROASBidPatches(values []ROASBidPatch) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value.Country) || value.Goal != nil && !validROASGoal(*value.Goal) || value.MaxBid != nil && !validBid(*value.MaxBid) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validRetentionBid(value RetentionBid) bool {
	return validCountry(value.Country) && validBid(value.BaseBid) && validResponseMaxBid(value.MaxBid)
}

func validRetentionBids(values []RetentionBid, response bool) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		maxValid := validBid(value.MaxBid)
		if response {
			maxValid = validResponseMaxBid(value.MaxBid)
		}
		if !validCountry(value.Country) || !validBid(value.BaseBid) || !maxValid || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validRetentionBidPatches(values []RetentionBidPatch) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value.Country) || value.BaseBid != nil && !validBid(*value.BaseBid) || !validBid(value.MaxBid) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validEventOptimizationInfo(value EventOptimizationInfo) bool {
	if !validText(string(value.EventOptimizationType), 255) {
		return false
	}
	for _, country := range value.Countries {
		if !validCountry(country) {
			return false
		}
	}
	return true
}

func validEventOptimizationBid(value EventOptimizationBid) bool {
	return validCountry(value.Country) && validBid(value.Bid)
}

func validEventOptimizationBids(values []EventOptimizationBid) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validEventOptimizationBid(value) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validEventOptimizationBidPatches(values []EventOptimizationBidPatch) bool {
	seen := make(map[CountryCode]struct{}, len(values))
	for _, value := range values {
		if !validCountry(value.Country) || value.Bid != nil && !validBid(*value.Bid) || duplicateCountry(seen, value.Country) {
			return false
		}
	}
	return true
}

func validSDKEventNamesInfo(value SDKEventNamesInfo) bool {
	if !validText(string(value.EventOptimizationType), 255) {
		return false
	}
	for _, name := range value.SDKEventNames {
		if !validText(string(name), 255) {
			return false
		}
	}
	return true
}

func duplicateCountry(seen map[CountryCode]struct{}, value CountryCode) bool {
	if _, exists := seen[value]; exists {
		return true
	}
	seen[value] = struct{}{}
	return false
}

func duplicateString(seen map[string]struct{}, value string) bool {
	if _, exists := seen[value]; exists {
		return true
	}
	seen[value] = struct{}{}
	return false
}

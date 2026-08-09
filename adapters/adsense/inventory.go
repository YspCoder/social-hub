package adsense

import (
	"context"

	"social-hub/pkg/socialhub"
)

type InventoryWorkflow interface {
	GetAdClient(context.Context, string, ...socialhub.CallOption) (*AdClient, error)
	ListAdClients(context.Context, ListRequest, ...socialhub.CallOption) (Page[AdClient], error)
	GetAdClientAdCode(context.Context, string, ...socialhub.CallOption) (*AdClientAdCode, error)
	GetAdUnit(context.Context, string, string, ...socialhub.CallOption) (*AdUnit, error)
	ListAdUnits(context.Context, string, ListRequest, ...socialhub.CallOption) (Page[AdUnit], error)
	GetAdUnitAdCode(context.Context, string, string, ...socialhub.CallOption) (*AdUnitAdCode, error)
	ListAdUnitCustomChannels(context.Context, string, string, ListRequest, ...socialhub.CallOption) (Page[CustomChannel], error)
	GetCustomChannel(context.Context, string, string, ...socialhub.CallOption) (*CustomChannel, error)
	ListCustomChannels(context.Context, string, ListRequest, ...socialhub.CallOption) (Page[CustomChannel], error)
	ListCustomChannelAdUnits(context.Context, string, string, ListRequest, ...socialhub.CallOption) (Page[AdUnit], error)
	GetURLChannel(context.Context, string, string, ...socialhub.CallOption) (*URLChannel, error)
	ListURLChannels(context.Context, string, ListRequest, ...socialhub.CallOption) (Page[URLChannel], error)
	GetSite(context.Context, string, ...socialhub.CallOption) (*Site, error)
	ListSites(context.Context, ListRequest, ...socialhub.CallOption) (Page[Site], error)
}

func (client *Client) GetAdClient(ctx context.Context, adClientID string, options ...socialhub.CallOption) (*AdClient, error) {
	const operation = "ad_client_get"
	name, err := client.adClientName(operation, adClientID)
	if err != nil {
		return nil, err
	}
	var output AdClient
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "ad client")
	}
	return &output, nil
}

func (client *Client) ListAdClients(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[AdClient], error) {
	const operation = "ad_clients_list"
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[AdClient]{}, err
	}
	var output struct {
		AdClients     []AdClient `json:"adClients"`
		NextPageToken string     `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/adclients", query, &output, options...); err != nil {
		return Page[AdClient]{}, err
	}
	for _, item := range output.AdClients {
		if !client.ownsAdClient(item.Name) {
			return Page[AdClient]{}, ownershipError(operation, "ad client")
		}
	}
	return Page[AdClient]{Items: output.AdClients, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetAdClientAdCode(ctx context.Context, adClientID string, options ...socialhub.CallOption) (*AdClientAdCode, error) {
	const operation = "ad_client_ad_code_get"
	name, err := client.adClientName(operation, adClientID)
	if err != nil {
		return nil, err
	}
	var output AdClientAdCode
	if err := client.getJSON(ctx, operation, "/v2/"+name+"/adcode", nil, &output, options...); err != nil {
		return nil, err
	}
	if output.AdCode == "" || len(output.Raw) == 0 {
		return nil, platformContractError(operation, "AdSense returned invalid ad client code")
	}
	return &output, nil
}

func (client *Client) GetAdUnit(ctx context.Context, adClientID, adUnitID string, options ...socialhub.CallOption) (*AdUnit, error) {
	const operation = "ad_unit_get"
	name, err := client.nestedName(operation, adClientID, "adunits", adUnitID)
	if err != nil {
		return nil, err
	}
	var output AdUnit
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "ad unit")
	}
	return &output, nil
}

func (client *Client) ListAdUnits(ctx context.Context, adClientID string, input ListRequest, options ...socialhub.CallOption) (Page[AdUnit], error) {
	const operation = "ad_units_list"
	parent, err := client.adClientName(operation, adClientID)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	var output struct {
		AdUnits       []AdUnit `json:"adUnits"`
		NextPageToken string   `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+parent+"/adunits", query, &output, options...); err != nil {
		return Page[AdUnit]{}, err
	}
	for _, item := range output.AdUnits {
		if !client.ownsResource(item.Name, parent, "adunits") {
			return Page[AdUnit]{}, ownershipError(operation, "ad unit")
		}
	}
	return Page[AdUnit]{Items: output.AdUnits, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetAdUnitAdCode(ctx context.Context, adClientID, adUnitID string, options ...socialhub.CallOption) (*AdUnitAdCode, error) {
	const operation = "ad_unit_ad_code_get"
	name, err := client.nestedName(operation, adClientID, "adunits", adUnitID)
	if err != nil {
		return nil, err
	}
	var output AdUnitAdCode
	if err := client.getJSON(ctx, operation, "/v2/"+name+"/adcode", nil, &output, options...); err != nil {
		return nil, err
	}
	if output.AdCode == "" || len(output.Raw) == 0 {
		return nil, platformContractError(operation, "AdSense returned invalid ad unit code")
	}
	return &output, nil
}

func (client *Client) ListAdUnitCustomChannels(ctx context.Context, adClientID, adUnitID string, input ListRequest, options ...socialhub.CallOption) (Page[CustomChannel], error) {
	const operation = "ad_unit_custom_channels_list"
	parent, err := client.nestedName(operation, adClientID, "adunits", adUnitID)
	if err != nil {
		return Page[CustomChannel]{}, err
	}
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[CustomChannel]{}, err
	}
	var output struct {
		CustomChannels []CustomChannel `json:"customChannels"`
		NextPageToken  string          `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+parent+":listLinkedCustomChannels", query, &output, options...); err != nil {
		return Page[CustomChannel]{}, err
	}
	for _, item := range output.CustomChannels {
		if !client.ownsNested(item.Name, "customchannels") {
			return Page[CustomChannel]{}, ownershipError(operation, "custom channel")
		}
	}
	return Page[CustomChannel]{Items: output.CustomChannels, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetCustomChannel(ctx context.Context, adClientID, channelID string, options ...socialhub.CallOption) (*CustomChannel, error) {
	const operation = "custom_channel_get"
	name, err := client.nestedName(operation, adClientID, "customchannels", channelID)
	if err != nil {
		return nil, err
	}
	var output CustomChannel
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "custom channel")
	}
	return &output, nil
}

func (client *Client) ListCustomChannels(ctx context.Context, adClientID string, input ListRequest, options ...socialhub.CallOption) (Page[CustomChannel], error) {
	const operation = "custom_channels_list"
	parent, err := client.adClientName(operation, adClientID)
	if err != nil {
		return Page[CustomChannel]{}, err
	}
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[CustomChannel]{}, err
	}
	var output struct {
		CustomChannels []CustomChannel `json:"customChannels"`
		NextPageToken  string          `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+parent+"/customchannels", query, &output, options...); err != nil {
		return Page[CustomChannel]{}, err
	}
	for _, item := range output.CustomChannels {
		if !client.ownsResource(item.Name, parent, "customchannels") {
			return Page[CustomChannel]{}, ownershipError(operation, "custom channel")
		}
	}
	return Page[CustomChannel]{Items: output.CustomChannels, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) ListCustomChannelAdUnits(ctx context.Context, adClientID, channelID string, input ListRequest, options ...socialhub.CallOption) (Page[AdUnit], error) {
	const operation = "custom_channel_ad_units_list"
	parent, err := client.nestedName(operation, adClientID, "customchannels", channelID)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[AdUnit]{}, err
	}
	var output struct {
		AdUnits       []AdUnit `json:"adUnits"`
		NextPageToken string   `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+parent+":listLinkedAdUnits", query, &output, options...); err != nil {
		return Page[AdUnit]{}, err
	}
	for _, item := range output.AdUnits {
		if !client.ownsNested(item.Name, "adunits") {
			return Page[AdUnit]{}, ownershipError(operation, "ad unit")
		}
	}
	return Page[AdUnit]{Items: output.AdUnits, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetURLChannel(ctx context.Context, adClientID, channelID string, options ...socialhub.CallOption) (*URLChannel, error) {
	const operation = "url_channel_get"
	name, err := client.nestedName(operation, adClientID, "urlchannels", channelID)
	if err != nil {
		return nil, err
	}
	var output URLChannel
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "URL channel")
	}
	return &output, nil
}

func (client *Client) ListURLChannels(ctx context.Context, adClientID string, input ListRequest, options ...socialhub.CallOption) (Page[URLChannel], error) {
	const operation = "url_channels_list"
	parent, err := client.adClientName(operation, adClientID)
	if err != nil {
		return Page[URLChannel]{}, err
	}
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[URLChannel]{}, err
	}
	var output struct {
		URLChannels   []URLChannel `json:"urlChannels"`
		NextPageToken string       `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+parent+"/urlchannels", query, &output, options...); err != nil {
		return Page[URLChannel]{}, err
	}
	for _, item := range output.URLChannels {
		if !client.ownsResource(item.Name, parent, "urlchannels") {
			return Page[URLChannel]{}, ownershipError(operation, "URL channel")
		}
	}
	return Page[URLChannel]{Items: output.URLChannels, NextPageToken: output.NextPageToken}, nil
}

func (client *Client) GetSite(ctx context.Context, siteID string, options ...socialhub.CallOption) (*Site, error) {
	const operation = "site_get"
	name, err := client.resourceName(operation, client.accountName(), "sites", siteID)
	if err != nil {
		return nil, err
	}
	var output Site
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "site")
	}
	return &output, nil
}

func (client *Client) ListSites(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Site], error) {
	const operation = "sites_list"
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[Site]{}, err
	}
	var output struct {
		Sites         []Site `json:"sites"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/sites", query, &output, options...); err != nil {
		return Page[Site]{}, err
	}
	for _, item := range output.Sites {
		if !client.ownsResource(item.Name, client.accountName(), "sites") {
			return Page[Site]{}, ownershipError(operation, "site")
		}
	}
	return Page[Site]{Items: output.Sites, NextPageToken: output.NextPageToken}, nil
}

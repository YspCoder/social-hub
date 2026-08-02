package patreon

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	campaignFields = "created_at,creation_name,currency,image_small_url,image_url,is_monthly,is_nsfw,name,patron_count,published_at,summary,url,vanity"
	memberFields   = "campaign_lifetime_support_cents,currently_entitled_amount_cents,full_name,last_charge_date,last_charge_status,patron_status,pledge_relationship_start,will_pay_amount_cents"
)

func (client *Client) GetCampaign(ctx context.Context, options ...socialhub.CallOption) (*Campaign, error) {
	api, err := client.requireAPI("get_campaign")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("get_campaign", "campaigns"); err != nil {
		return nil, err
	}
	var response campaignResponse
	if err := api.JSON(ctx, http.MethodGet, resourcePath("campaigns", client.campaignID), campaignQuery(nil), nil, &response, options...); err != nil {
		return nil, err
	}
	if !client.validCampaign(response.Data, client.campaignID) {
		return nil, platformError("get_campaign", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	campaign := mapCampaign(response.Data)
	return &campaign, nil
}

func (client *Client) ListCampaigns(ctx context.Context, maximum int, cursor string, options ...socialhub.CallOption) (socialhub.Page[Campaign], error) {
	query, err := pageQuery(maximum, cursor)
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	api, err := client.requireAPI("list_campaigns")
	if err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	if err := client.requireScopes("list_campaigns", "campaigns"); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	var response campaignListResponse
	if err := api.JSON(ctx, http.MethodGet, "/campaigns", campaignQuery(query), nil, &response, options...); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	items := make([]Campaign, 0, len(response.Data))
	for _, campaign := range response.Data {
		if client.validCampaign(campaign, "") {
			items = append(items, mapCampaign(campaign))
		}
	}
	page := socialhub.Page[Campaign]{Items: items}
	if err := setNextCursor(&page, response.Meta.Pagination.Cursors.Next); err != nil {
		return socialhub.Page[Campaign]{}, err
	}
	return page, nil
}

func (client *Client) GetMember(ctx context.Context, memberID string, options ...socialhub.CallOption) (*Member, error) {
	if !validResourceID(memberID) {
		return nil, invalidArgument("get_member", "Member ID is invalid")
	}
	api, err := client.requireAPI("get_member")
	if err != nil {
		return nil, err
	}
	if err := client.requireScopes("get_member", "campaigns.members"); err != nil {
		return nil, err
	}
	var response memberResponse
	if err := api.JSON(ctx, http.MethodGet, resourcePath("members", memberID), client.memberQuery(nil), nil, &response, options...); err != nil {
		return nil, err
	}
	if !client.validMember(response.Data, memberID) {
		return nil, platformError("get_member", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	member := mapMember(response.Data)
	return &member, nil
}

func (client *Client) ListMembers(ctx context.Context, maximum int, cursor string, options ...socialhub.CallOption) (socialhub.Page[Member], error) {
	query, err := pageQuery(maximum, cursor)
	if err != nil {
		return socialhub.Page[Member]{}, err
	}
	api, err := client.requireAPI("list_members")
	if err != nil {
		return socialhub.Page[Member]{}, err
	}
	if err := client.requireScopes("list_members", "campaigns.members"); err != nil {
		return socialhub.Page[Member]{}, err
	}
	var response memberListResponse
	path := resourcePath("campaigns", client.campaignID) + "/members"
	if err := api.JSON(ctx, http.MethodGet, path, client.memberQuery(query), nil, &response, options...); err != nil {
		return socialhub.Page[Member]{}, err
	}
	items := make([]Member, 0, len(response.Data))
	for _, member := range response.Data {
		if client.validMember(member, "") {
			items = append(items, mapMember(member))
		}
	}
	page := socialhub.Page[Member]{Items: items}
	if err := setNextCursor(&page, response.Meta.Pagination.Cursors.Next); err != nil {
		return socialhub.Page[Member]{}, err
	}
	return page, nil
}

func (client *Client) validCampaign(campaign campaignResource, expectedID string) bool {
	return campaign.Type == "campaign" && validResourceID(campaign.ID) && (expectedID == "" || campaign.ID == expectedID)
}

func (client *Client) validMember(member memberResource, expectedID string) bool {
	if member.Type != "member" || !validResourceID(member.ID) || expectedID != "" && member.ID != expectedID {
		return false
	}
	return member.Relationships.Campaign.Data == nil || member.Relationships.Campaign.Data.ID == client.campaignID
}

func campaignQuery(query url.Values) url.Values {
	if query == nil {
		query = url.Values{}
	}
	query.Set("fields[campaign]", campaignFields)
	query.Set("include", "null")
	return query
}

func (client *Client) memberQuery(query url.Values) url.Values {
	if query == nil {
		query = url.Values{}
	}
	fields := memberFields
	if scopeGranted(client.scopes, "campaigns.members[email]") {
		fields += ",email"
	}
	query.Set("fields[member]", fields)
	query.Set("include", "null")
	return query
}

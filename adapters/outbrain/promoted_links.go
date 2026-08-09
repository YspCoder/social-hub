package outbrain

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListPromotedLinks(ctx context.Context, campaignID string, input ListPromotedLinksRequest, options ...socialhub.CallOption) (PromotedLinkPage, error) {
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return PromotedLinkPage{}, err
	}
	return client.listPromotedLinksUnchecked(ctx, campaignID, input, options...)
}

func (client *Client) listPromotedLinksUnchecked(ctx context.Context, campaignID string, input ListPromotedLinksRequest, options ...socialhub.CallOption) (PromotedLinkPage, error) {
	if !validPathID(campaignID) || !validListPromotedLinks(input) {
		return PromotedLinkPage{}, invalidArgument("list_promoted_links", "campaign ID or list filters are invalid")
	}
	query := make(url.Values)
	if input.Enabled != nil {
		query.Set("enabled", strconv.FormatBool(*input.Enabled))
	}
	if len(input.Statuses) > 0 {
		query.Set("statuses", strings.Join(input.Statuses, ","))
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	if input.ImageWidth > 0 {
		query.Set("promotedLinkImageWidth", strconv.Itoa(input.ImageWidth))
	}
	if input.ImageHeight > 0 {
		query.Set("promotedLinkImageHeight", strconv.Itoa(input.ImageHeight))
	}
	var envelope struct {
		PromotedLinks []PromotedLink `json:"promotedLinks"`
		Count         int            `json:"count"`
		TotalCount    int            `json:"totalCount"`
	}
	path := "campaigns/" + url.PathEscape(campaignID) + "/promotedLinks"
	if err := client.getJSON(ctx, "list_promoted_links", path, query, &envelope, options...); err != nil {
		return PromotedLinkPage{}, err
	}
	if envelope.Count != len(envelope.PromotedLinks) || envelope.TotalCount < envelope.Count {
		return PromotedLinkPage{}, platformContractError("list_promoted_links", "PromotedLink counts do not match results")
	}
	for _, link := range envelope.PromotedLinks {
		if err := validatePromotedLink("list_promoted_links", link, campaignID, ""); err != nil {
			return PromotedLinkPage{}, err
		}
	}
	return PromotedLinkPage{Items: envelope.PromotedLinks, Count: envelope.Count, TotalCount: envelope.TotalCount}, nil
}

func (client *Client) GetPromotedLink(ctx context.Context, campaignID, promotedLinkID string, options ...socialhub.CallOption) (PromotedLink, error) {
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return PromotedLink{}, err
	}
	return client.getPromotedLinkUnchecked(ctx, campaignID, promotedLinkID, options...)
}

func (client *Client) getPromotedLinkUnchecked(ctx context.Context, campaignID, promotedLinkID string, options ...socialhub.CallOption) (PromotedLink, error) {
	if !validPathID(campaignID) || !validPathID(promotedLinkID) {
		return PromotedLink{}, invalidArgument("get_promoted_link", "campaign or PromotedLink ID is invalid")
	}
	var link PromotedLink
	if err := client.getJSON(ctx, "get_promoted_link", "promotedLinks/"+url.PathEscape(promotedLinkID), nil, &link, options...); err != nil {
		return PromotedLink{}, err
	}
	if err := validatePromotedLink("get_promoted_link", link, campaignID, promotedLinkID); err != nil {
		return PromotedLink{}, err
	}
	return link, nil
}

func (client *Client) CreatePromotedLink(ctx context.Context, campaignID string, input CreatePromotedLinkRequest, options ...socialhub.CallOption) (PromotedLink, error) {
	if !validPathID(campaignID) || !validCreatePromotedLink(input) {
		return PromotedLink{}, invalidArgument("create_promoted_link", "campaign ID or PromotedLink fields are invalid")
	}
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return PromotedLink{}, err
	}
	if campaign.Enabled || campaign.LiveStatus.CampaignOnAir {
		return PromotedLink{}, invalidArgument("create_promoted_link", "parent Campaign must be disabled and off air")
	}
	payload := struct {
		Text          string   `json:"text"`
		URL           string   `json:"url"`
		Enabled       bool     `json:"enabled"`
		CPC           *float64 `json:"cpc,omitempty"`
		SectionName   string   `json:"sectionName,omitempty"`
		Description   string   `json:"description,omitempty"`
		ImageMetadata struct {
			URL         string `json:"url"`
			AIGenerated bool   `json:"aiGenerated"`
		} `json:"imageMetadata"`
	}{
		Text: input.Text, URL: input.URL, Enabled: false, CPC: input.CPC,
		SectionName: input.SectionName, Description: input.Description,
	}
	payload.ImageMetadata.URL = input.ImageURL
	payload.ImageMetadata.AIGenerated = input.AIGenerated
	var link PromotedLink
	path := "campaigns/" + url.PathEscape(campaignID) + "/promotedLinks"
	if err := client.postJSON(ctx, "create_promoted_link", path, payload, &link, options...); err != nil {
		return PromotedLink{}, err
	}
	if err := validatePromotedLink("create_promoted_link", link, campaignID, ""); err != nil {
		return PromotedLink{}, err
	}
	if link.Enabled || link.OnAirStatus.OnAir || link.Text != input.Text || link.URL != input.URL {
		return PromotedLink{}, platformContractError("create_promoted_link", "PromotedLink was not created in the requested disabled state")
	}
	return link, nil
}

func (client *Client) SetPromotedLinkEnabled(ctx context.Context, campaignID, promotedLinkID string, enabled bool, options ...socialhub.CallOption) error {
	campaign, err := client.GetCampaign(ctx, campaignID, options...)
	if err != nil {
		return err
	}
	link, err := client.getPromotedLinkUnchecked(ctx, campaignID, promotedLinkID, options...)
	if err != nil {
		return err
	}
	if link.Enabled == enabled {
		return nil
	}
	if enabled && (!campaign.Enabled || campaign.AutoArchived || link.Archived || !link.Approved()) {
		return invalidArgument("set_promoted_link_enabled", "enabling requires an enabled Campaign and an unarchived, approved PromotedLink")
	}
	payload := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	return client.putJSON(ctx, "set_promoted_link_enabled", "promotedLinks/"+url.PathEscape(promotedLinkID), payload, nil, options...)
}

func (client *Client) UpdatePromotedLinkCPCs(ctx context.Context, campaignID string, updates []PromotedLinkCPCUpdate, options ...socialhub.CallOption) ([]PromotedLinkUpdateResult, error) {
	if !validPathID(campaignID) || len(updates) == 0 || len(updates) > 100 {
		return nil, invalidArgument("update_promoted_link_cpcs", "campaign ID and 1-100 updates are required")
	}
	seen := make(map[string]struct{}, len(updates))
	for _, update := range updates {
		if !validPathID(update.ID) || update.CPC <= 0 || update.CPC > 1_000_000 {
			return nil, invalidArgument("update_promoted_link_cpcs", "PromotedLink ID or CPC is invalid")
		}
		if _, duplicate := seen[update.ID]; duplicate {
			return nil, invalidArgument("update_promoted_link_cpcs", "duplicate PromotedLink ID")
		}
		seen[update.ID] = struct{}{}
	}
	if _, err := client.GetCampaign(ctx, campaignID, options...); err != nil {
		return nil, err
	}
	links, err := client.listPromotedLinksUnchecked(ctx, campaignID, ListPromotedLinksRequest{Limit: 500}, options...)
	if err != nil {
		return nil, err
	}
	if links.TotalCount != len(links.Items) {
		return nil, invalidArgument("update_promoted_link_cpcs", "Campaign has more than 500 PromotedLinks and cannot be completely ownership-checked")
	}
	owned := make(map[string]struct{}, len(links.Items))
	for _, link := range links.Items {
		owned[link.ID] = struct{}{}
	}
	for id := range seen {
		if _, found := owned[id]; !found {
			return nil, platformError("update_promoted_link_cpcs", socialhub.CodePermissionDenied, socialhub.ClassUserAction, nil)
		}
	}
	var results []PromotedLinkUpdateResult
	path := "campaigns/" + url.PathEscape(campaignID) + "/promotedLinks"
	if err := client.putJSON(ctx, "update_promoted_link_cpcs", path, updates, &results, options...); err != nil {
		return nil, err
	}
	if len(results) != len(updates) {
		return nil, platformContractError("update_promoted_link_cpcs", "update result count does not match request")
	}
	for _, result := range results {
		id := result.ID
		if id == "" {
			id = result.PromotedLink.ID
		}
		if !equalFold(result.OperationStatus.Status, "SUCCESS") {
			return nil, platformContractError("update_promoted_link_cpcs", "PromotedLink update did not succeed")
		}
		if _, found := seen[id]; !found {
			return nil, platformContractError("update_promoted_link_cpcs", "update response contains an unexpected PromotedLink")
		}
		if err := validatePromotedLink("update_promoted_link_cpcs", result.PromotedLink, campaignID, id); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func validatePromotedLink(operation string, link PromotedLink, campaignID, requestedID string) error {
	if !validPathID(link.ID) || (requestedID != "" && link.ID != requestedID) || link.CampaignID != campaignID ||
		!validText(link.Text, 2048) || !validDestinationURL(link.URL) {
		return platformContractError(operation, "PromotedLink response ownership or fields are invalid")
	}
	return nil
}

func validListPromotedLinks(input ListPromotedLinksRequest) bool {
	if !validPage(input.Limit, input.Offset, 500) || (input.Sort != "" && input.Sort != "creationTime" && input.Sort != "-creationTime") ||
		input.ImageWidth < 0 || input.ImageWidth > 4096 || input.ImageHeight < 0 || input.ImageHeight > 4096 || len(input.Statuses) > 16 {
		return false
	}
	for _, status := range input.Statuses {
		if status != "APPROVED" && status != "PENDING" && status != "REJECTED" {
			return false
		}
	}
	return true
}

func validCreatePromotedLink(input CreatePromotedLinkRequest) bool {
	return validText(input.Text, 2048) && validDestinationURL(input.URL) && validPositive(input.CPC) &&
		(input.SectionName == "" || validText(input.SectionName, 1024)) &&
		(input.Description == "" || validText(input.Description, 4096)) && validDestinationURL(input.ImageURL)
}

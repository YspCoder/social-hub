package outbrain

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListMarketers(ctx context.Context, options ...socialhub.CallOption) ([]Marketer, error) {
	var envelope struct {
		Marketers []Marketer `json:"marketers"`
		Count     int        `json:"count"`
	}
	query := url.Values{"extraFields": {"Account"}}
	if err := client.getJSON(ctx, "list_marketers", "marketers", query, &envelope, options...); err != nil {
		return nil, err
	}
	if envelope.Count != len(envelope.Marketers) {
		return nil, platformContractError("list_marketers", "marketer count does not match results")
	}
	for _, marketer := range envelope.Marketers {
		if !validPathID(marketer.ID) || !validText(marketer.Name, 1024) {
			return nil, platformContractError("list_marketers", "invalid Marketer response")
		}
	}
	return envelope.Marketers, nil
}

func (client *Client) GetMarketer(ctx context.Context, options ...socialhub.CallOption) (Marketer, error) {
	var marketer Marketer
	if err := client.getJSON(ctx, "get_marketer", "marketers/"+url.PathEscape(client.marketerID), url.Values{"extraFields": {"Account"}}, &marketer, options...); err != nil {
		return Marketer{}, err
	}
	if marketer.ID != client.marketerID || !validText(marketer.Name, 1024) {
		return Marketer{}, platformContractError("get_marketer", "configured Marketer does not match response")
	}
	return marketer, nil
}

func (client *Client) ValidateConfiguredMarketer(ctx context.Context, options ...socialhub.CallOption) (Marketer, error) {
	marketers, err := client.ListMarketers(ctx, options...)
	if err != nil {
		return Marketer{}, err
	}
	for _, marketer := range marketers {
		if marketer.ID == client.marketerID {
			return client.GetMarketer(ctx, options...)
		}
	}
	return Marketer{}, platformError("validate_marketer", socialhub.CodePermissionDenied, socialhub.ClassUserAction, nil)
}

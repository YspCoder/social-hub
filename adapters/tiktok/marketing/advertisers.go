package marketing

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (*Advertiser, error) {
	const operation = "advertiser_get"
	query := url.Values{}
	if err := setJSONQuery(query, "advertiser_ids", []string{client.advertiserID}, operation); err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		List []Advertiser `json:"list"`
	}]
	header, err := client.getJSON(ctx, operation, "/v1.3/advertiser/info/", query, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if len(data.List) != 1 || requireResourceID(operation, client.advertiserID, data.List[0].ID) != nil {
		return nil, platformContractError(operation, "TikTok did not return the configured advertiser")
	}
	return &data.List[0], nil
}

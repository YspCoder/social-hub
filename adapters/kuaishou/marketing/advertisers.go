package marketing

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (*Advertiser, error) {
	const operation = "advertiser_get"
	body := map[string]any{"advertiser_id": client.advertiserID}
	var response apiEnvelope[Advertiser]
	header, err := client.postJSON(ctx, operation, "/v1/advertiser/info", body, &response, options...)
	if err != nil {
		return nil, err
	}
	advertiser, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := requireAdvertiser(operation, client.advertiserID, advertiser.AdvertiserID); err != nil {
		return nil, err
	}
	advertiser.AdvertiserID = client.advertiserID
	return advertiser, nil
}

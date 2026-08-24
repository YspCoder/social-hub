package tencentads

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

var advertiserFields = []string{
	"account_id", "daily_budget", "system_status", "corporation_name", "corporate_brand_name",
	"system_industry_id", "agency_account_id", "area_code",
}

func (client *Client) GetAdvertiser(ctx context.Context, options ...socialhub.CallOption) (*Advertiser, error) {
	const operation = "advertiser_get"
	query := url.Values{
		"account_id": {strconv.FormatInt(client.advertiserID, 10)}, "page": {"1"}, "page_size": {"1"},
	}
	if err := setJSONQuery(query, "fields", advertiserFields, operation); err != nil {
		return nil, err
	}
	var response apiEnvelope[struct {
		List     []Advertiser `json:"list"`
		PageInfo *pageInfo    `json:"page_info"`
	}]
	header, err := client.requestJSON(ctx, operation, http.MethodGet, "/advertiser/get", query, nil, &response, options...)
	if err != nil {
		return nil, err
	}
	data, err := requireEnvelope(operation, response, header)
	if err != nil {
		return nil, err
	}
	if err := validatePageInfo(operation, data.PageInfo); err != nil {
		return nil, err
	}
	if len(data.List) != 1 || data.List[0].AccountID != client.advertiserID {
		return nil, platformContractError(operation, "Tencent Ads did not return exactly the configured advertiser")
	}
	return &data.List[0], nil
}

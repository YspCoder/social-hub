package ads

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListFundingInstruments(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (socialhub.Page[FundingInstrument], error) {
	const operation = "funding_instruments_list"
	if !validList(input) {
		return socialhub.Page[FundingInstrument]{}, invalidArgument(operation, "pagination is invalid")
	}
	path := client.accountPath("funding_instruments")
	var response listResponse[FundingInstrument]
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return socialhub.Page[FundingInstrument]{}, err
	}
	for index := range response.Data {
		if !validResourceID(response.Data[index].ID) {
			return socialhub.Page[FundingInstrument]{}, platformContractError(operation, "Reddit returned an invalid Funding Instrument ID")
		}
		response.Data[index].AdAccountID = client.adAccountID
	}
	cursor, err := client.pageCursor(operation, path, response.Pagination.NextURL)
	if err != nil {
		return socialhub.Page[FundingInstrument]{}, err
	}
	return page(response.Data, cursor), nil
}

func (client *Client) GetFundingInstrument(ctx context.Context, id string, options ...socialhub.CallOption) (*FundingInstrument, error) {
	return client.getFundingInstrument(ctx, "funding_instrument_get", id, options...)
}

func (client *Client) getFundingInstrument(ctx context.Context, operation, id string, options ...socialhub.CallOption) (*FundingInstrument, error) {
	if !validResourceID(id) {
		return nil, invalidArgument(operation, "Funding Instrument ID must be numeric")
	}
	path := client.accountPath("funding_instruments")
	query := url.Values{"funding_instrument_ids": {id}, "page.size": {"2"}}
	var response listResponse[FundingInstrument]
	if _, err := client.getJSON(ctx, operation, path, query, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, platformError(operation, socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if len(response.Data) != 1 || response.Data[0].ID != id {
		return nil, platformContractError(operation, "Reddit returned a mismatched Funding Instrument selection")
	}
	response.Data[0].AdAccountID = client.adAccountID
	return &response.Data[0], nil
}

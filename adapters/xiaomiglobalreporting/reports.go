package xiaomiglobalreporting

import (
	"bytes"
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type reportQueryWire struct {
	Page         int         `json:"page,omitempty"`
	PageSize     int         `json:"pageSize,omitempty"`
	AdType       AdType      `json:"adType"`
	AccountIDs   []int64     `json:"accountIds"`
	CampaignIDs  []int64     `json:"adCampaignIds,omitempty"`
	AdGroupIDs   []int64     `json:"adGroupIds,omitempty"`
	CreativeIDs  []int64     `json:"adCreativeIds,omitempty"`
	PlacementIDs []string    `json:"adTagIds,omitempty"`
	Regions      []string    `json:"regions,omitempty"`
	PublisherIDs []string    `json:"medias,omitempty"`
	Dimensions   []Dimension `json:"dimensions"`
	Begin        string      `json:"begin"`
	End          string      `json:"end"`
	Language     Language    `json:"lang"`
}

type nameQueryWire struct {
	AccountIDs  []int64 `json:"accountIds"`
	CampaignIDs []int64 `json:"adCampaignIds,omitempty"`
	AdGroupIDs  []int64 `json:"adGroupIds,omitempty"`
	CreativeIDs []int64 `json:"adCreativeIds,omitempty"`
}

func (client *Client) Query(ctx context.Context, input ReportQuery, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "query_report"
	accountIDs, err := client.validateReportQuery(operation, input)
	if err != nil {
		return ReportPage{}, err
	}
	wire := reportQueryWire{
		Page: input.Page, PageSize: input.PageSize, AdType: input.AdType,
		AccountIDs: accountIDs, CampaignIDs: append([]int64(nil), input.CampaignIDs...),
		AdGroupIDs: append([]int64(nil), input.AdGroupIDs...), CreativeIDs: append([]int64(nil), input.CreativeIDs...),
		PlacementIDs: append([]string(nil), input.PlacementIDs...), Regions: append([]string(nil), input.Regions...),
		PublisherIDs: append([]string(nil), input.PublisherIDs...), Dimensions: append([]Dimension(nil), input.Dimensions...),
		Begin: string(input.Begin) + "T00:00:00.000Z", End: string(input.End) + "T23:59:59.999Z", Language: input.Language,
	}
	result, requestUID, err := client.doJSON(ctx, operation, "/foreign/data/queryData", wire, options...)
	if err != nil {
		return ReportPage{}, err
	}
	page, err := decodeReportPage(result, accountIDs)
	if err != nil {
		return ReportPage{}, platformContractError(operation, err.Error())
	}
	page.RequestUID = requestUID
	return page, nil
}

func (client *Client) QueryNames(ctx context.Context, input NameQuery, options ...socialhub.CallOption) (NameDirectory, error) {
	const operation = "query_names"
	accountIDs, err := client.validateNameQuery(operation, input)
	if err != nil {
		return NameDirectory{}, err
	}
	wire := nameQueryWire{
		AccountIDs: accountIDs, CampaignIDs: append([]int64(nil), input.CampaignIDs...),
		AdGroupIDs: append([]int64(nil), input.AdGroupIDs...), CreativeIDs: append([]int64(nil), input.CreativeIDs...),
	}
	result, requestUID, err := client.doJSON(ctx, operation, "/foreign/data/queryDataName", wire, options...)
	if err != nil {
		return NameDirectory{}, err
	}
	directory, err := decodeNameDirectory(result, accountIDs)
	if err != nil {
		return NameDirectory{}, platformContractError(operation, err.Error())
	}
	directory.RequestUID = requestUID
	return directory, nil
}

func decodeReportPage(raw json.RawMessage, accountIDs []int64) (ReportPage, error) {
	var wire struct {
		Current json.RawMessage `json:"current"`
		Pages   json.RawMessage `json:"pages"`
		Records json.RawMessage `json:"records"`
		Size    json.RawMessage `json:"size"`
		Total   json.RawMessage `json:"total"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire.Current) == 0 || len(wire.Pages) == 0 ||
		len(wire.Records) == 0 || len(wire.Size) == 0 || len(wire.Total) == 0 {
		return ReportPage{}, errInvalidReportResult
	}
	current, err := decodeNonnegativeInt64(wire.Current, 1_000_000_000)
	if err != nil {
		return ReportPage{}, err
	}
	pages, err := decodeNonnegativeInt64(wire.Pages, 1_000_000_000)
	if err != nil {
		return ReportPage{}, err
	}
	size, err := decodeNonnegativeInt64(wire.Size, maximumPageSize)
	if err != nil {
		return ReportPage{}, err
	}
	total, err := decodeNonnegativeInt64(wire.Total, 1_000_000_000_000)
	if err != nil {
		return ReportPage{}, err
	}
	var recordData []json.RawMessage
	if bytes.Equal(bytes.TrimSpace(wire.Records), []byte("null")) {
		return ReportPage{}, errInvalidReportResult
	}
	if err := json.Unmarshal(wire.Records, &recordData); err != nil || len(recordData) > maximumReportRows {
		return ReportPage{}, errInvalidReportResult
	}
	if (len(recordData) > 0 && current == 0) || (pages > 0 && current > pages) ||
		(total > 0 && pages == 0) || total < int64(len(recordData)) || (size > 0 && int64(len(recordData)) > size) {
		return ReportPage{}, errInvalidReportResult
	}
	authorized := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		authorized[accountID] = struct{}{}
	}
	records := make([]ReportRow, len(recordData))
	for index, data := range recordData {
		var row ReportRow
		if err := json.Unmarshal(data, &row); err != nil || !validReportRow(row, authorized) {
			return ReportPage{}, errInvalidReportRow
		}
		records[index] = row
	}
	return ReportPage{Current: current, Pages: pages, Size: size, Total: total, Records: records}, nil
}

func decodeNameDirectory(raw json.RawMessage, accountIDs []int64) (NameDirectory, error) {
	var wire struct {
		Accounts json.RawMessage `json:"adCount"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire.Accounts) == 0 {
		return NameDirectory{}, errInvalidNameResult
	}
	if bytes.Equal(bytes.TrimSpace(wire.Accounts), []byte("null")) {
		return NameDirectory{}, errInvalidNameResult
	}
	var accounts []AccountNames
	if err := json.Unmarshal(wire.Accounts, &accounts); err != nil {
		return NameDirectory{}, errInvalidNameResult
	}
	authorized := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		authorized[accountID] = struct{}{}
	}
	if !validNameDirectory(accounts, authorized) {
		return NameDirectory{}, errInvalidNameResult
	}
	return NameDirectory{Accounts: accounts}, nil
}

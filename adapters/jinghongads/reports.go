package jinghongads

import (
	"context"
	"encoding/json"
	"net/http"

	"social-hub/pkg/socialhub"
)

const (
	advertiserReportPath = "/openapi/v2/reports/advertiser/query"
	campaignReportPath   = "/openapi/v2/reports/campaign/query"
	adGroupReportPath    = "/openapi/v2/reports/adgroup/query"
	creativeReportPath   = "/openapi/v2/reports/creative/query"
)

type reportRequestWire struct {
	AdvertiserID    string               `json:"advertiser_id"`
	TimeGranularity TimeGranularity      `json:"time_granularity,omitempty"`
	Filtering       any                  `json:"filtering,omitempty"`
	Page            int                  `json:"page,omitempty"`
	PageSize        int                  `json:"page_size,omitempty"`
	StartDate       Date                 `json:"start_date"`
	EndDate         Date                 `json:"end_date"`
	OrderField      string               `json:"order_field,omitempty"`
	OrderType       OrderType            `json:"order_type,omitempty"`
	TopN            int                  `json:"topn,omitempty"`
	FlowResource    int                  `json:"flow_resource,omitempty"`
	CampaignType    CampaignType         `json:"campaign_type,omitempty"`
	MetricFilters   []metricFilterWire   `json:"index_screen_list,omitempty"`
	Dimension       *dimensionFilterWire `json:"dimension_type,omitempty"`
	TimeLine        string               `json:"time_line,omitempty"`
	GroupBy         []GroupBy            `json:"group_by,omitempty"`
}

type metricFilterWire struct {
	Metric    Metric   `json:"index_screen"`
	Type      int      `json:"type"`
	LowValue  *Decimal `json:"low_value,omitempty"`
	HighValue *Decimal `json:"up_value,omitempty"`
}

type dimensionFilterWire struct {
	Dimension string   `json:"dimension"`
	Data      []string `json:"data"`
}

type campaignReportFilterWire struct {
	CampaignIDs  []string `json:"campaign_ids,omitempty"`
	CampaignName string   `json:"campaign_name,omitempty"`
	ProductTypes []string `json:"product_types,omitempty"`
}

type adGroupReportFilterWire struct {
	CampaignIDs          []string `json:"campaign_ids,omitempty"`
	CampaignName         string   `json:"campaign_name,omitempty"`
	AdGroupIDs           []string `json:"adgroup_ids,omitempty"`
	AdGroupName          string   `json:"adgroup_name,omitempty"`
	ProductTypes         []string `json:"product_types,omitempty"`
	AppIDs               []string `json:"app_ids,omitempty"`
	AppChannelPackageIDs []string `json:"app_channel_package_ids,omitempty"`
	PlacementName        string   `json:"placement_name,omitempty"`
	Pricings             []string `json:"pricings,omitempty"`
}

type creativeReportFilterWire struct {
	CampaignIDs   []string `json:"campaign_ids,omitempty"`
	CampaignName  string   `json:"campaign_name,omitempty"`
	AdGroupIDs    []string `json:"adgroup_ids,omitempty"`
	AdGroupName   string   `json:"adgroup_name,omitempty"`
	CreativeIDs   []string `json:"creative_ids,omitempty"`
	PlacementName string   `json:"placement_name,omitempty"`
	Pricings      []string `json:"pricings,omitempty"`
}

func (client *Client) AdvertiserReport(ctx context.Context, input AdvertiserReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "advertiser_report"
	if err := validateReportBase(operation, input.ReportBase, reportAdvertiser); err != nil {
		return ReportPage{}, err
	}
	return client.queryReport(ctx, operation, advertiserReportPath, reportWire(client.advertiserID, input.ReportBase, reportAdvertiser), options...)
}

func (client *Client) CampaignReport(ctx context.Context, input CampaignReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "campaign_report"
	if err := validateReportBase(operation, input.ReportBase, reportCampaign); err != nil {
		return ReportPage{}, err
	}
	if !validateCampaignReportFilter(input.Filter) {
		return ReportPage{}, invalidArgument(operation, "Campaign report filters are invalid")
	}
	wire := reportWire(client.advertiserID, input.ReportBase, reportCampaign)
	if !emptyCampaignReportFilter(input.Filter) {
		wire.Filtering = campaignReportFilterWire{
			CampaignIDs: append([]string(nil), input.Filter.CampaignIDs...), CampaignName: input.Filter.CampaignName,
			ProductTypes: append([]string(nil), input.Filter.ProductTypes...),
		}
	}
	return client.queryReport(ctx, operation, campaignReportPath, wire, options...)
}

func (client *Client) AdGroupReport(ctx context.Context, input AdGroupReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "adgroup_report"
	if err := validateReportBase(operation, input.ReportBase, reportAdGroup); err != nil {
		return ReportPage{}, err
	}
	if !validateAdGroupReportFilter(input.Filter) {
		return ReportPage{}, invalidArgument(operation, "Ad Group report filters are invalid")
	}
	wire := reportWire(client.advertiserID, input.ReportBase, reportAdGroup)
	if !emptyAdGroupReportFilter(input.Filter) {
		wire.Filtering = adGroupReportFilterWire{
			CampaignIDs: append([]string(nil), input.Filter.CampaignIDs...), CampaignName: input.Filter.CampaignName,
			AdGroupIDs: append([]string(nil), input.Filter.AdGroupIDs...), AdGroupName: input.Filter.AdGroupName,
			ProductTypes: append([]string(nil), input.Filter.ProductTypes...), AppIDs: append([]string(nil), input.Filter.AppIDs...),
			AppChannelPackageIDs: append([]string(nil), input.Filter.AppChannelPackageIDs...), PlacementName: input.Filter.PlacementName,
			Pricings: append([]string(nil), input.Filter.Pricings...),
		}
	}
	return client.queryReport(ctx, operation, adGroupReportPath, wire, options...)
}

func (client *Client) CreativeReport(ctx context.Context, input CreativeReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "creative_report"
	if err := validateReportBase(operation, input.ReportBase, reportCreative); err != nil {
		return ReportPage{}, err
	}
	if !validateCreativeReportFilter(input.Filter) {
		return ReportPage{}, invalidArgument(operation, "Creative report filters are invalid")
	}
	wire := reportWire(client.advertiserID, input.ReportBase, reportCreative)
	if !emptyCreativeReportFilter(input.Filter) {
		wire.Filtering = creativeReportFilterWire{
			CampaignIDs: append([]string(nil), input.Filter.CampaignIDs...), CampaignName: input.Filter.CampaignName,
			AdGroupIDs: append([]string(nil), input.Filter.AdGroupIDs...), AdGroupName: input.Filter.AdGroupName,
			CreativeIDs: append([]string(nil), input.Filter.CreativeIDs...), PlacementName: input.Filter.PlacementName,
			Pricings: append([]string(nil), input.Filter.Pricings...),
		}
	}
	return client.queryReport(ctx, operation, creativeReportPath, wire, options...)
}

func reportWire(advertiserID string, input ReportBase, kind reportKind) reportRequestWire {
	return reportRequestWire{
		AdvertiserID: advertiserID, TimeGranularity: input.TimeGranularity, Page: input.Page, PageSize: input.PageSize,
		StartDate: input.StartDate, EndDate: input.EndDate, OrderField: input.OrderField, OrderType: input.OrderType,
		TopN: input.TopN, FlowResource: input.FlowResource, CampaignType: input.CampaignType,
		MetricFilters: metricFiltersWire(input.MetricFilters, kind), Dimension: cloneDimensionWire(input.Dimension),
		TimeLine: input.TimeLine, GroupBy: append([]GroupBy(nil), input.GroupBy...),
	}
}

func metricFiltersWire(filters []MetricFilter, kind reportKind) []metricFilterWire {
	if len(filters) == 0 {
		return nil
	}
	result := make([]metricFilterWire, len(filters))
	for index, filter := range filters {
		result[index] = metricFilterWire{
			Metric: filter.Metric, Type: metricFilterType(kind, filter.Mode),
			LowValue: filter.LowValue, HighValue: filter.HighValue,
		}
	}
	return result
}

func metricFilterType(kind reportKind, mode MetricFilterMode) int {
	typeValue := 1 + 3*int(kind)
	switch mode {
	case MetricLessOrEqual:
		return typeValue + 1
	case MetricBetween:
		return typeValue + 2
	default:
		return typeValue
	}
}

func cloneDimensionWire(input *DimensionFilter) *dimensionFilterWire {
	if input == nil {
		return nil
	}
	return &dimensionFilterWire{Dimension: input.Dimension, Data: append([]string(nil), input.Data...)}
}

func (client *Client) queryReport(ctx context.Context, operation, path string, input reportRequestWire, options ...socialhub.CallOption) (ReportPage, error) {
	if err := client.requireAdvertiser(operation); err != nil {
		return ReportPage{}, err
	}
	var response struct {
		PageInfo struct {
			Page        json.RawMessage `json:"page"`
			PageSize    json.RawMessage `json:"page_size"`
			TotalNum    json.RawMessage `json:"total_num"`
			TotalNumber json.RawMessage `json:"total_number"`
			TotalPages  json.RawMessage `json:"total_page"`
		} `json:"page_info"`
		Rows    []json.RawMessage `json:"list"`
		Summary json.RawMessage   `json:"list_summary"`
	}
	if err := client.doJSON(ctx, operation, ScopeReport, http.MethodPost, path, input, &response, options...); err != nil {
		return ReportPage{}, err
	}
	if len(response.Rows) > 10_000 || len(response.Summary) == 0 {
		return ReportPage{}, platformContractError(operation, "Jinghong returned an invalid report page")
	}
	page, err := decodeNonnegativeInt(response.PageInfo.Page, 10_000)
	if err != nil || page == 0 {
		return ReportPage{}, platformContractError(operation, "Jinghong returned invalid report page metadata")
	}
	pageSize, err := decodeNonnegativeInt(response.PageInfo.PageSize, 10_000)
	if err != nil || pageSize == 0 || pageSize < len(response.Rows) {
		return ReportPage{}, platformContractError(operation, "Jinghong returned invalid report page metadata")
	}
	totalNumber, validTotal := decodeReportTotal(response.PageInfo.TotalNum, response.PageInfo.TotalNumber)
	if !validTotal || totalNumber < len(response.Rows) {
		return ReportPage{}, platformContractError(operation, "Jinghong returned invalid report totals")
	}
	totalPages, err := decodeNonnegativeInt(response.PageInfo.TotalPages, 1_000_000_000)
	if err != nil || totalNumber > 0 && (totalPages == 0 || page > totalPages) {
		return ReportPage{}, platformContractError(operation, "Jinghong returned invalid report totals")
	}
	rows := make([]ReportRow, len(response.Rows))
	for index, raw := range response.Rows {
		if err := json.Unmarshal(raw, &rows[index]); err != nil || rows[index] == nil || !validReportRow(rows[index]) {
			return ReportPage{}, platformContractError(operation, "Jinghong returned an invalid dynamic report row")
		}
	}
	var summary ReportRow
	if err := json.Unmarshal(response.Summary, &summary); err != nil || summary == nil || !validReportRow(summary) {
		return ReportPage{}, platformContractError(operation, "Jinghong returned an invalid report summary")
	}
	return ReportPage{
		Rows: rows, Summary: summary,
		PageInfo: PageInfo{Page: page, PageSize: pageSize, TotalNumber: totalNumber, TotalPages: totalPages},
		HasMore:  totalPages > 0 && page < totalPages,
	}, nil
}

func decodeReportTotal(totalNum, totalNumber json.RawMessage) (int, bool) {
	values := make([]int, 0, 2)
	for _, raw := range []json.RawMessage{totalNum, totalNumber} {
		if len(raw) == 0 {
			continue
		}
		value, err := decodeNonnegativeInt(raw, 1_000_000_000)
		if err != nil {
			return 0, false
		}
		values = append(values, value)
	}
	if len(values) == 0 || len(values) == 2 && values[0] != values[1] {
		return 0, false
	}
	return values[0], true
}

func emptyCampaignReportFilter(filter CampaignReportFilter) bool {
	return len(filter.CampaignIDs) == 0 && filter.CampaignName == "" && len(filter.ProductTypes) == 0
}

func emptyAdGroupReportFilter(filter AdGroupReportFilter) bool {
	return len(filter.CampaignIDs) == 0 && filter.CampaignName == "" && len(filter.AdGroupIDs) == 0 && filter.AdGroupName == "" &&
		len(filter.ProductTypes) == 0 && len(filter.AppIDs) == 0 && len(filter.AppChannelPackageIDs) == 0 &&
		filter.PlacementName == "" && len(filter.Pricings) == 0
}

func emptyCreativeReportFilter(filter CreativeReportFilter) bool {
	return len(filter.CampaignIDs) == 0 && filter.CampaignName == "" && len(filter.AdGroupIDs) == 0 && filter.AdGroupName == "" &&
		len(filter.CreativeIDs) == 0 && filter.PlacementName == "" && len(filter.Pricings) == 0
}

package xiaohongshureporting

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

const (
	offlineAccountPath    = "/api/open/jg/data/report/offline/account"
	offlineCampaignPath   = "/api/open/jg/data/report/offline/campaign"
	offlineUnitPath       = "/api/open/jg/data/report/offline/unit"
	offlineCreativePath   = "/api/open/jg/data/report/offline/creative"
	offlineKeywordPath    = "/api/open/jg/data/report/offline/keyword"
	offlineNotePath       = "/api/open/jg/data/report/offline/note"
	offlineSPUPath        = "/api/open/jg/data/report/offline/spu"
	offlineSearchWordPath = "/api/open/jg/data/report/offline/search/word"
	realtimeAccountPath   = "/api/open/jg/data/report/realtime/account"
	realtimeCampaignPath  = "/api/open/jg/data/report/realtime/campaign"
	realtimeUnitPath      = "/api/open/jg/data/report/realtime/unit"
	realtimeCreativePath  = "/api/open/jg/data/report/realtime/creativity"
	realtimeKeywordPath   = "/api/open/jg/data/report/realtime/keyword"
	realtimeTargetPath    = "/api/open/jg/data/report/realtime/target"
)

type offlineReportWire struct {
	AdvertiserID    uint64         `json:"advertiser_id"`
	StartDate       Date           `json:"start_date"`
	EndDate         Date           `json:"end_date"`
	TimeUnit        TimeUnit       `json:"time_unit,omitempty"`
	MarketingTarget []int          `json:"marketing_target,omitempty"`
	BiddingStrategy []int          `json:"bidding_strategy,omitempty"`
	OptimizeTarget  []int          `json:"optimize_target,omitempty"`
	Placement       []int          `json:"placement,omitempty"`
	PromotionTarget []int          `json:"promotion_target,omitempty"`
	Programmatic    []int          `json:"programmatic,omitempty"`
	BuildType       []int          `json:"build_type,omitempty"`
	DeliveryMode    []int          `json:"delivery_mode,omitempty"`
	SplitColumns    []string       `json:"split_columns,omitempty"`
	SortColumn      string         `json:"sort_column,omitempty"`
	Sort            SortOrder      `json:"sort,omitempty"`
	PageNum         int            `json:"page_num,omitempty"`
	PageSize        int            `json:"page_size,omitempty"`
	DataCaliber     DataCaliber    `json:"data_caliber,omitempty"`
	Filters         []FilterClause `json:"filters,omitempty"`
}

type offlineSimpleWire struct {
	AdvertiserID uint64    `json:"advertiser_id"`
	StartDate    Date      `json:"start_date"`
	EndDate      Date      `json:"end_date"`
	TimeUnit     TimeUnit  `json:"time_unit,omitempty"`
	SortColumn   string    `json:"sort_column,omitempty"`
	Sort         SortOrder `json:"sort,omitempty"`
	PageNum      int       `json:"page_num,omitempty"`
	PageSize     int       `json:"page_size,omitempty"`
}

type offlineSearchWordWire struct {
	AdvertiserID    uint64      `json:"advertiser_id"`
	StartDate       Date        `json:"start_date"`
	EndDate         Date        `json:"end_date"`
	TimeUnit        TimeUnit    `json:"time_unit,omitempty"`
	MarketingTarget []int       `json:"marketing_target,omitempty"`
	BiddingStrategy []int       `json:"bidding_strategy,omitempty"`
	OptimizeTarget  []int       `json:"optimize_target,omitempty"`
	Placement       []int       `json:"placement,omitempty"`
	PromotionTarget []int       `json:"promotion_target,omitempty"`
	Programmatic    []int       `json:"programmatic,omitempty"`
	BuildType       []int       `json:"build_type,omitempty"`
	SortColumn      string      `json:"sort_column,omitempty"`
	Sort            SortOrder   `json:"sort,omitempty"`
	PageNum         int         `json:"page_num,omitempty"`
	PageSize        int         `json:"page_size,omitempty"`
	DataCaliber     DataCaliber `json:"data_caliber,omitempty"`
}

type realtimeAccountWire struct {
	AdvertiserID   uint64      `json:"advertiser_id"`
	StartDate      Date        `json:"start_date"`
	EndDate        Date        `json:"end_date"`
	NeedHourlyData bool        `json:"need_hourly_data,omitempty"`
	DataCaliber    DataCaliber `json:"data_caliber,omitempty"`
}

type realtimeCampaignWire struct {
	AdvertiserID            uint64      `json:"advertiser_id"`
	StartDate               Date        `json:"start_date"`
	EndDate                 Date        `json:"end_date"`
	SortColumn              string      `json:"sort_column,omitempty"`
	Sort                    SortOrder   `json:"sort,omitempty"`
	PageNum                 int         `json:"page_num,omitempty"`
	PageSize                int         `json:"page_size,omitempty"`
	MarketingTargetList     []int       `json:"marketing_target_list,omitempty"`
	CampaignFilterState     int         `json:"campaign_filter_state,omitempty"`
	CampaignCreateBeginTime string      `json:"campaign_create_begin_time,omitempty"`
	CampaignCreateEndTime   string      `json:"campaign_create_end_time,omitempty"`
	PlacementList           []int       `json:"placement_list,omitempty"`
	LimitDayBudgetList      []int       `json:"limit_day_budget_list,omitempty"`
	OptimizeTargetList      []int       `json:"optimize_target_list,omitempty"`
	BuildTypeList           []int       `json:"build_type_list,omitempty"`
	BiddingStrategyList     []int       `json:"bidding_strategy_list,omitempty"`
	ConstraintTypeList      []int       `json:"constraint_type_list,omitempty"`
	PromotionTargetList     []int       `json:"promotion_target_list,omitempty"`
	CombineAuditStatus      int         `json:"combine_audit_status,omitempty"`
	MigrationStatusList     []int       `json:"migration_status_list,omitempty"`
	Name                    string      `json:"name,omitempty"`
	ID                      int64       `json:"id,omitempty"`
	DataCaliber             DataCaliber `json:"data_caliber,omitempty"`
	NeedHourlyData          bool        `json:"need_hourly_data,omitempty"`
}

type realtimeUnitWire struct {
	AdvertiserID        uint64      `json:"advertiser_id"`
	StartDate           Date        `json:"start_date"`
	EndDate             Date        `json:"end_date"`
	PageNum             int         `json:"page_num,omitempty"`
	PageSize            int         `json:"page_size,omitempty"`
	SortColumn          string      `json:"sort_column,omitempty"`
	Sort                SortOrder   `json:"sort,omitempty"`
	MarketingTargetList []int       `json:"marketing_target_list,omitempty"`
	UnitFilterState     int         `json:"unit_filter_state,omitempty"`
	UnitCreateBeginTime string      `json:"unit_create_begin_time,omitempty"`
	UnitCreateEndTime   string      `json:"unit_create_end_time,omitempty"`
	PlacementList       []int       `json:"placement_list,omitempty"`
	BiddingStrategyList []int       `json:"bidding_strategy_list,omitempty"`
	PromotionTargetList []int       `json:"promotion_target_list,omitempty"`
	CombineAuditStatus  int         `json:"combine_audit_status,omitempty"`
	Name                string      `json:"name,omitempty"`
	ID                  int64       `json:"id,omitempty"`
	DataCaliber         DataCaliber `json:"data_caliber,omitempty"`
	NeedHourlyData      bool        `json:"need_hourly_data,omitempty"`
}

type realtimeCreativeWire struct {
	AdvertiserID              uint64      `json:"advertiser_id"`
	StartDate                 Date        `json:"start_date"`
	EndDate                   Date        `json:"end_date"`
	PageNum                   int         `json:"page_num,omitempty"`
	PageSize                  int         `json:"page_size,omitempty"`
	SortColumn                string      `json:"sort_column,omitempty"`
	Sort                      SortOrder   `json:"sort,omitempty"`
	PlacementList             []int       `json:"placement_list,omitempty"`
	CreativityFilterState     int         `json:"creativity_filter_state,omitempty"`
	CreativityCreateBeginTime string      `json:"creativity_create_begin_time,omitempty"`
	CreativityCreateEndTime   string      `json:"creativity_create_end_time,omitempty"`
	ConversionType            int         `json:"conversion_type,omitempty"`
	ProgrammaticList          []int       `json:"programmatic_list,omitempty"`
	CreativityAuditState      int         `json:"creativity_audit_state,omitempty"`
	Name                      string      `json:"name,omitempty"`
	ID                        int64       `json:"id,omitempty"`
	DataCaliber               DataCaliber `json:"data_caliber,omitempty"`
	NeedHourlyData            bool        `json:"need_hourly_data,omitempty"`
}

type realtimeKeywordWire struct {
	AdvertiserID       uint64      `json:"advertiser_id"`
	StartDate          Date        `json:"start_date"`
	EndDate            Date        `json:"end_date"`
	PageNum            int         `json:"page_num,omitempty"`
	PageSize           int         `json:"page_size,omitempty"`
	SortColumn         string      `json:"sort_column,omitempty"`
	Sort               SortOrder   `json:"sort,omitempty"`
	KeywordFilterState int         `json:"keyword_filter_state,omitempty"`
	UseBidStrategy     int         `json:"use_bid_strategy,omitempty"`
	KeywordName        string      `json:"keyword_name,omitempty"`
	CampaignName       string      `json:"campaign_name,omitempty"`
	UnitName           string      `json:"unit_name,omitempty"`
	DataCaliber        DataCaliber `json:"data_caliber,omitempty"`
	NeedHourlyData     bool        `json:"need_hourly_data,omitempty"`
}

type realtimeTargetWire struct {
	AdvertiserID        uint64    `json:"advertiser_id"`
	StartDate           Date      `json:"start_date"`
	EndDate             Date      `json:"end_date"`
	PageNum             int       `json:"page_num,omitempty"`
	PageSize            int       `json:"page_size,omitempty"`
	SortColumn          string    `json:"sort_column,omitempty"`
	Sort                SortOrder `json:"sort,omitempty"`
	Name                string    `json:"name,omitempty"`
	MarketingTargetList []int     `json:"marketing_target_list,omitempty"`
	NeedHourlyData      bool      `json:"need_hourly_data,omitempty"`
}

func (client *Client) OfflineAccount(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_account", offlineAccountPath, input, options...)
}

func (client *Client) OfflineCampaign(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_campaign", offlineCampaignPath, input, options...)
}

func (client *Client) OfflineUnit(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_unit", offlineUnitPath, input, options...)
}

func (client *Client) OfflineCreative(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_creative", offlineCreativePath, input, options...)
}

func (client *Client) OfflineKeyword(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_keyword", offlineKeywordPath, input, options...)
}

func (client *Client) OfflineNote(ctx context.Context, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineReport(ctx, "offline_note", offlineNotePath, input, options...)
}

func (client *Client) offlineReport(ctx context.Context, operation, path string, input OfflineReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	if err := validateOfflineReport(operation, input); err != nil {
		return ReportPage{}, err
	}
	wire := offlineReportWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate, TimeUnit: input.TimeUnit,
		MarketingTarget: cloneInts(input.MarketingTarget), BiddingStrategy: cloneInts(input.BiddingStrategy),
		OptimizeTarget: cloneInts(input.OptimizeTarget), Placement: cloneInts(input.Placement),
		PromotionTarget: cloneInts(input.PromotionTarget), Programmatic: cloneInts(input.Programmatic),
		BuildType: cloneInts(input.BuildType), DeliveryMode: cloneInts(input.DeliveryMode),
		SplitColumns: append([]string(nil), input.SplitColumns...), SortColumn: input.SortColumn, Sort: input.Sort,
		PageNum: input.PageNum, PageSize: input.PageSize, DataCaliber: input.DataCaliber,
		Filters: cloneFilters(input.Filters),
	}
	return client.queryReport(ctx, operation, path, wire, reportShape{maxRows: 500}, options...)
}

func (client *Client) OfflineSPU(ctx context.Context, input OfflineSimpleReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	return client.offlineSimpleReport(ctx, "offline_spu", offlineSPUPath, input, options...)
}

func (client *Client) offlineSimpleReport(ctx context.Context, operation, path string, input OfflineSimpleReportRequest, options ...socialhub.CallOption) (ReportPage, error) {
	if err := validateOfflineSimpleReport(operation, input); err != nil {
		return ReportPage{}, err
	}
	wire := offlineSimpleWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		TimeUnit: input.TimeUnit, SortColumn: input.SortColumn, Sort: input.Sort,
		PageNum: input.PageNum, PageSize: input.PageSize,
	}
	return client.queryReport(ctx, operation, path, wire, reportShape{maxRows: 500}, options...)
}

func (client *Client) OfflineSearchWord(ctx context.Context, input OfflineSearchWordRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "offline_search_word"
	if err := validateOfflineSearchWord(operation, input); err != nil {
		return ReportPage{}, err
	}
	wire := offlineSearchWordWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate, TimeUnit: input.TimeUnit,
		MarketingTarget: cloneInts(input.MarketingTarget), BiddingStrategy: cloneInts(input.BiddingStrategy),
		OptimizeTarget: cloneInts(input.OptimizeTarget), Placement: cloneInts(input.Placement),
		PromotionTarget: cloneInts(input.PromotionTarget), Programmatic: cloneInts(input.Programmatic),
		BuildType: cloneInts(input.BuildType), SortColumn: input.SortColumn, Sort: input.Sort,
		PageNum: input.PageNum, PageSize: input.PageSize, DataCaliber: input.DataCaliber,
	}
	return client.queryReport(ctx, operation, offlineSearchWordPath, wire, reportShape{maxRows: 500}, options...)
}

func (client *Client) RealtimeAccount(ctx context.Context, input RealtimeAccountRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_account"
	if err := validateRealtimeBase(operation, input.StartDate, input.EndDate, 0, 0, "", "", input.NeedHourlyData, client.clock); err != nil {
		return ReportPage{}, err
	}
	if !validDataCaliber(input.DataCaliber) {
		return ReportPage{}, invalidArgument(operation, "data caliber must be billing time or conversion time")
	}
	wire := realtimeAccountWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		NeedHourlyData: input.NeedHourlyData, DataCaliber: input.DataCaliber,
	}
	return client.queryReport(ctx, operation, realtimeAccountPath, wire, reportShape{account: true, maxRows: 24}, options...)
}

func (client *Client) RealtimeCampaign(ctx context.Context, input RealtimeCampaignRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_campaign"
	if err := validateRealtimeCampaign(operation, input, client.clock); err != nil {
		return ReportPage{}, err
	}
	wire := realtimeCampaignWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		SortColumn: input.SortColumn, Sort: input.Sort, PageNum: input.PageNum, PageSize: input.PageSize,
		MarketingTargetList: cloneInts(input.MarketingTargetList), CampaignFilterState: input.CampaignFilterState,
		CampaignCreateBeginTime: input.CampaignCreateBeginTime, CampaignCreateEndTime: input.CampaignCreateEndTime,
		PlacementList: cloneInts(input.PlacementList), LimitDayBudgetList: cloneInts(input.LimitDayBudgetList),
		OptimizeTargetList: cloneInts(input.OptimizeTargetList), BuildTypeList: cloneInts(input.BuildTypeList),
		BiddingStrategyList: cloneInts(input.BiddingStrategyList), ConstraintTypeList: cloneInts(input.ConstraintTypeList),
		PromotionTargetList: cloneInts(input.PromotionTargetList), CombineAuditStatus: input.CombineAuditStatus,
		MigrationStatusList: cloneInts(input.MigrationStatusList), Name: input.Name, ID: input.ID,
		DataCaliber: input.DataCaliber, NeedHourlyData: input.NeedHourlyData,
	}
	return client.queryReport(ctx, operation, realtimeCampaignPath, wire, reportShape{listKey: "campaign_dtos", maxRows: 100}, options...)
}

func (client *Client) RealtimeUnit(ctx context.Context, input RealtimeUnitRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_unit"
	if err := validateRealtimeUnit(operation, input, client.clock); err != nil {
		return ReportPage{}, err
	}
	wire := realtimeUnitWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		PageNum: input.PageNum, PageSize: input.PageSize, SortColumn: input.SortColumn, Sort: input.Sort,
		MarketingTargetList: cloneInts(input.MarketingTargetList), UnitFilterState: input.UnitFilterState,
		UnitCreateBeginTime: input.UnitCreateBeginTime, UnitCreateEndTime: input.UnitCreateEndTime,
		PlacementList: cloneInts(input.PlacementList), BiddingStrategyList: cloneInts(input.BiddingStrategyList),
		PromotionTargetList: cloneInts(input.PromotionTargetList), CombineAuditStatus: input.CombineAuditStatus,
		Name: input.Name, ID: input.ID, DataCaliber: input.DataCaliber, NeedHourlyData: input.NeedHourlyData,
	}
	return client.queryReport(ctx, operation, realtimeUnitPath, wire, reportShape{listKey: "unit_dtos", maxRows: 100}, options...)
}

func (client *Client) RealtimeCreative(ctx context.Context, input RealtimeCreativeRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_creative"
	if err := validateRealtimeCreative(operation, input, client.clock); err != nil {
		return ReportPage{}, err
	}
	wire := realtimeCreativeWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		PageNum: input.PageNum, PageSize: input.PageSize, SortColumn: input.SortColumn, Sort: input.Sort,
		PlacementList: cloneInts(input.PlacementList), CreativityFilterState: input.CreativityFilterState,
		CreativityCreateBeginTime: input.CreativityCreateBeginTime, CreativityCreateEndTime: input.CreativityCreateEndTime,
		ConversionType: input.ConversionType, ProgrammaticList: cloneInts(input.ProgrammaticList),
		CreativityAuditState: input.CreativityAuditState, Name: input.Name, ID: input.ID,
		DataCaliber: input.DataCaliber, NeedHourlyData: input.NeedHourlyData,
	}
	return client.queryReport(ctx, operation, realtimeCreativePath, wire, reportShape{listKey: "creativity_dtos", maxRows: 100}, options...)
}

func (client *Client) RealtimeKeyword(ctx context.Context, input RealtimeKeywordRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_keyword"
	if err := validateRealtimeKeyword(operation, input, client.clock); err != nil {
		return ReportPage{}, err
	}
	wire := realtimeKeywordWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		PageNum: input.PageNum, PageSize: input.PageSize, SortColumn: input.SortColumn, Sort: input.Sort,
		KeywordFilterState: input.KeywordFilterState, UseBidStrategy: input.UseBidStrategy,
		KeywordName: input.KeywordName, CampaignName: input.CampaignName, UnitName: input.UnitName,
		DataCaliber: input.DataCaliber, NeedHourlyData: input.NeedHourlyData,
	}
	return client.queryReport(ctx, operation, realtimeKeywordPath, wire, reportShape{listKey: "keyword_dtos", maxRows: 100}, options...)
}

func (client *Client) RealtimeTarget(ctx context.Context, input RealtimeTargetRequest, options ...socialhub.CallOption) (ReportPage, error) {
	const operation = "realtime_target"
	if err := validateRealtimeTarget(operation, input, client.clock); err != nil {
		return ReportPage{}, err
	}
	wire := realtimeTargetWire{
		AdvertiserID: client.advertiserID, StartDate: input.StartDate, EndDate: input.EndDate,
		PageNum: input.PageNum, PageSize: input.PageSize, SortColumn: input.SortColumn, Sort: input.Sort,
		Name: input.Name, MarketingTargetList: cloneInts(input.MarketingTargetList), NeedHourlyData: input.NeedHourlyData,
	}
	return client.queryReport(ctx, operation, realtimeTargetPath, wire, reportShape{listKey: "target_dtos", maxRows: 100}, options...)
}

type reportShape struct {
	listKey string
	account bool
	maxRows int
}

func (client *Client) queryReport(ctx context.Context, operation, path string, input any, shape reportShape, options ...socialhub.CallOption) (ReportPage, error) {
	raw, requestID, err := client.doJSON(ctx, operation, path, input, options...)
	if err != nil {
		return ReportPage{}, err
	}
	page, err := decodeReportPage(raw, requestID, shape)
	if err != nil {
		return ReportPage{}, platformContractError(operation, err.Error())
	}
	return page, nil
}

func decodeReportPage(raw json.RawMessage, requestID string, shape reportShape) (ReportPage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil || root == nil {
		return ReportPage{}, errInvalidReportEnvelope
	}
	page := ReportPage{RequestID: requestID}
	nested := map[string]json.RawMessage{}
	data, hasData := root["data"]
	if hasData && !isJSONNull(data) {
		if err := json.Unmarshal(data, &nested); err != nil || nested == nil {
			return ReportPage{}, errInvalidReportEnvelope
		}
		if shape.account {
			account, err := decodeReportRow(data, false)
			if err != nil {
				return ReportPage{}, err
			}
			page.Account = account
		}
	}
	if shape.account && (!hasData || isJSONNull(data)) {
		return ReportPage{}, errInvalidReportEnvelope
	}
	listRaw := nested["data_list"]
	if shape.listKey != "" {
		listRaw = root[shape.listKey]
	}
	if !shape.account && len(listRaw) == 0 {
		return ReportPage{}, errInvalidReportEnvelope
	}
	if len(listRaw) > 0 && !isJSONNull(listRaw) {
		rows, err := decodeReportRows(listRaw, shape.maxRows)
		if err != nil {
			return ReportPage{}, err
		}
		page.Rows = rows
	}
	aggregationRaw := nested["aggregation_data"]
	if len(aggregationRaw) == 0 {
		aggregationRaw = root["total_data"]
	}
	if len(aggregationRaw) > 0 && !isJSONNull(aggregationRaw) {
		aggregation, err := decodeReportRow(aggregationRaw, true)
		if err != nil {
			return ReportPage{}, err
		}
		page.Aggregation = aggregation
	}
	if hourlyRaw := root["hourly_data"]; len(hourlyRaw) > 0 && !isJSONNull(hourlyRaw) {
		hourly, err := decodeReportRows(hourlyRaw, 48)
		if err != nil {
			return ReportPage{}, err
		}
		page.Hourly = hourly
	}
	pageRaw := root["page"]
	if len(pageRaw) == 0 {
		pageRaw = nested["page"]
	}
	if len(pageRaw) > 0 && !isJSONNull(pageRaw) {
		if err := decodePageInfo(pageRaw, &page.Page); err != nil {
			return ReportPage{}, err
		}
	}
	if totalRaw := nested["total_count"]; len(totalRaw) > 0 && page.Page.TotalCount == 0 {
		total, err := decodeNonnegativeInt64(totalRaw)
		if err != nil {
			return ReportPage{}, errInvalidReportPage
		}
		page.Page.TotalCount = total
	}
	if page.Page.TotalCount > 0 && page.Page.TotalCount < int64(len(page.Rows)) {
		return ReportPage{}, errInvalidReportPage
	}
	return page, nil
}

func decodeReportRows(raw json.RawMessage, maximum int) ([]ReportRow, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) > maximum {
		return nil, errInvalidReportRows
	}
	rows := make([]ReportRow, len(items))
	for index, item := range items {
		row, err := decodeReportRow(item, false)
		if err != nil {
			return nil, err
		}
		rows[index] = row
	}
	return rows, nil
}

func decodeReportRow(raw json.RawMessage, allowEmpty bool) (ReportRow, error) {
	var row ReportRow
	if err := json.Unmarshal(raw, &row); err != nil || !validReportRow(row, allowEmpty) {
		return nil, errInvalidReportRow
	}
	return row, nil
}

func decodePageInfo(raw json.RawMessage, output *PageInfo) error {
	var wire struct {
		PageIndex  json.RawMessage `json:"page_index"`
		PageNum    json.RawMessage `json:"page_num"`
		TotalCount json.RawMessage `json:"total_count"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return errInvalidReportPage
	}
	pageRaw := wire.PageIndex
	if len(pageRaw) == 0 {
		pageRaw = wire.PageNum
	}
	if len(pageRaw) > 0 {
		pageIndex, err := decodeNonnegativeInt64(pageRaw)
		if err != nil {
			return errInvalidReportPage
		}
		output.PageIndex = pageIndex
	}
	if len(wire.TotalCount) > 0 {
		total, err := decodeNonnegativeInt64(wire.TotalCount)
		if err != nil {
			return errInvalidReportPage
		}
		output.TotalCount = total
	}
	return nil
}

func cloneInts(values []int) []int { return append([]int(nil), values...) }

func cloneFilters(filters []FilterClause) []FilterClause {
	cloned := make([]FilterClause, len(filters))
	for index, filter := range filters {
		cloned[index] = FilterClause{Column: filter.Column, Operator: filter.Operator, Values: append([]string(nil), filter.Values...)}
	}
	return cloned
}

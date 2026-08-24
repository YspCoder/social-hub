package kakaomoment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

const maxReportValueBytes = 1 << 20

type ConfigStatus string

const (
	ConfigOn      ConfigStatus = "ON"
	ConfigOff     ConfigStatus = "OFF"
	ConfigDeleted ConfigStatus = "DEL"
)

type Company struct {
	BusinessRegistrationNumber string `json:"businessRegistrationNumber"`
	Name                       string `json:"name"`
}

type AdAccount struct {
	ID                int64        `json:"id"`
	Name              string       `json:"name"`
	Advertiser        *Company     `json:"advertiser"`
	Type              string       `json:"type"`
	Config            ConfigStatus `json:"config"`
	IsAdminStop       bool         `json:"isAdminStop"`
	IsOutOfBalance    bool         `json:"isOutOfBalance"`
	StatusDescription string       `json:"statusDescription"`
	BizWalletID       int64        `json:"bizWalletId"`
}

type Balance struct {
	ID          int64 `json:"id"`
	BizWalletID int64 `json:"bizWalletId"`
	Cash        int64 `json:"cash"`
	FreeCash    int64 `json:"freeCash"`
}

type CampaignTypeGoal struct {
	CampaignType string `json:"campaignType"`
	Goal         string `json:"goal"`
}

type Objective struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Campaign struct {
	ID                      int64             `json:"id"`
	Name                    string            `json:"name"`
	CampaignTypeGoal        *CampaignTypeGoal `json:"campaignTypeGoal"`
	Objective               *Objective        `json:"objective"`
	DailyBudgetAmount       *int64            `json:"dailyBudgetAmount"`
	Config                  ConfigStatus      `json:"config"`
	UserConfig              ConfigStatus      `json:"userConfig"`
	StatusDescription       string            `json:"statusDescription"`
	TrackID                 string            `json:"trackId"`
	AdAccountID             int64             `json:"adAccountId"`
	Status                  []string          `json:"status"`
	SystemConfig            string            `json:"systemConfig"`
	KCLID                   bool              `json:"kclid"`
	IsDailyBudgetAmountOver bool              `json:"isDailyBudgetAmountOver"`
}

// CampaignCreate covers the stable common Campaign fields. Kakao creates the
// resource ON; CreateCampaignThenPause immediately requests OFF afterwards.
type CampaignCreate struct {
	Name              string
	CampaignTypeGoal  CampaignTypeGoal
	Objective         *Objective
	DailyBudgetAmount *int64
	TrackID           string
	KCLID             *bool
}

type CampaignUpdate struct {
	ID      int64
	Name    *string
	TrackID *string
	KCLID   *bool
}

type AdGroup struct {
	ID                      int64           `json:"id"`
	Name                    string          `json:"name"`
	Config                  ConfigStatus    `json:"config"`
	UserConfig              ConfigStatus    `json:"userConfig"`
	SystemConfig            string          `json:"systemConfig"`
	Pacing                  string          `json:"pacing"`
	PricingType             string          `json:"pricingType"`
	BidAmount               int64           `json:"bidAmount"`
	BidStrategy             string          `json:"bidStrategy"`
	BidStrategyTarget       json.RawMessage `json:"bidStrategyTarget"`
	StatusDescription       string          `json:"statusDescription"`
	Status                  []string        `json:"status"`
	OptimizationStatus      []string        `json:"optimizationStatus"`
	DeviceTypes             []string        `json:"deviceTypes"`
	AdServingCategories     []string        `json:"adServingCategories"`
	SectionCategories       []string        `json:"sectionCategories"`
	Placements              []string        `json:"placements"`
	Targeting               json.RawMessage `json:"targeting"`
	Schedule                json.RawMessage `json:"schedule"`
	MessageSendingInfo      json.RawMessage `json:"messageSendingInfo"`
	Campaign                *Campaign       `json:"campaign"`
	ProfileID               string          `json:"profileId"`
	UseWiFiOnly             bool            `json:"useWifiOnly"`
	CreativeCount           int64           `json:"creativeCount"`
	AllAvailableDeviceType  bool            `json:"allAvailableDeviceType"`
	AllAvailablePlacement   bool            `json:"allAvailablePlacement"`
	Adult                   bool            `json:"adult"`
	SmartMessage            *bool           `json:"smartMessage"`
	DailyBudgetAmount       *int64          `json:"dailyBudgetAmount"`
	TotalBudget             *int64          `json:"totalBudget"`
	TotalBudgetWithVAT      *int64          `json:"totalBudgetWithVAT"`
	IsDailyBudgetAmountOver bool            `json:"isDailyBudgetAmountOver"`
	IsValidPeriod           bool            `json:"isValidPeriod"`
	CreatedDate             string          `json:"createdDate"`
	LastModifiedDate        string          `json:"lastModifiedDate"`
}

type Creative struct {
	ID                int64           `json:"id"`
	CreativeID        int64           `json:"creativeId"`
	AdGroupID         int64           `json:"adGroupId"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Format            string          `json:"format"`
	Config            ConfigStatus    `json:"config"`
	SystemConfig      string          `json:"systemConfig"`
	ReviewStatus      string          `json:"reviewStatus"`
	CreativeStatus    string          `json:"creativeStatus"`
	StatusDescription string          `json:"statusDescription"`
	PCLandingURL      string          `json:"pcLandingUrl"`
	MobileLandingURL  string          `json:"mobileLandingUrl"`
	ResponsiveURL     string          `json:"rspvLandingUrl"`
	FrequencyCapType  string          `json:"frequencyCapType"`
	FrequencyCap      int             `json:"frequencyCap"`
	Image             json.RawMessage `json:"image"`
	LandingInfo       json.RawMessage `json:"landingInfo"`
	Video             json.RawMessage `json:"video"`
	MessageElement    json.RawMessage `json:"messageElement"`
	RejectedReason    json.RawMessage `json:"rejectedReason"`
	CreatedDate       string          `json:"createdDate"`
	LastModifiedDate  string          `json:"lastModifiedDate"`
	AgeVerification   bool            `json:"ageVerification"`
	HasExpandable     bool            `json:"hasExpandable"`
}

type DatePreset string

const (
	DateToday      DatePreset = "TODAY"
	DateYesterday  DatePreset = "YESTERDAY"
	DateLast7Days  DatePreset = "LAST_7DAY"
	DateLast14Days DatePreset = "LAST_14DAY"
	DateLast30Days DatePreset = "LAST_30DAY"
	DateThisMonth  DatePreset = "THIS_MONTH"
	DateLastMonth  DatePreset = "LAST_MONTH"
)

type ReportRequest struct {
	DatePreset    DatePreset
	Start         string
	End           string
	TimeUnit      string
	Level         string
	Dimension     string
	MetricsGroups []string
}

type EntityReportRequest struct {
	IDs []int64
	ReportRequest
}

// ReportValue preserves provider numbers and strings without float64 coercion.
type ReportValue struct {
	raw json.RawMessage
}

func (value *ReportValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > maxReportValueBytes || !json.Valid(data) {
		return fmt.Errorf("kakaomoment: invalid report value")
	}
	value.raw = append(value.raw[:0], data...)
	return nil
}

func (value ReportValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ReportValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ReportValue) IsNull() bool {
	return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null"))
}

func (value ReportValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ReportValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("kakaomoment: decode target and report value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ReportRow struct {
	Start      string                 `json:"start"`
	End        string                 `json:"end"`
	Dimensions map[string]ReportValue `json:"dimensions"`
	Metrics    map[string]ReportValue `json:"metrics"`
}

type ReportResult struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      []ReportRow `json:"data"`
	RequestID string      `json:"-"`
}

type AccountWorkflow interface {
	GetAdAccount(context.Context, ...socialhub.CallOption) (*AdAccount, error)
	GetBalance(context.Context, ...socialhub.CallOption) (*Balance, error)
}

type CampaignWorkflow interface {
	ListCampaigns(context.Context, ConfigStatus, ...socialhub.CallOption) ([]Campaign, error)
	GetCampaign(context.Context, int64, ...socialhub.CallOption) (*Campaign, error)
	CreateCampaignThenPause(context.Context, CampaignCreate, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, CampaignUpdate, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignDailyBudget(context.Context, int64, *int64, ...socialhub.CallOption) error
	SetCampaignConfig(context.Context, int64, ConfigStatus, ...socialhub.CallOption) error
	DeleteCampaign(context.Context, int64, ...socialhub.CallOption) error
}

type AdGroupWorkflow interface {
	ListAdGroups(context.Context, int64, []ConfigStatus, ...socialhub.CallOption) ([]AdGroup, error)
	GetAdGroup(context.Context, int64, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupDailyBudget(context.Context, int64, int64, ...socialhub.CallOption) error
	SetAdGroupBidAmount(context.Context, int64, int64, ...socialhub.CallOption) error
	SetAdGroupConfig(context.Context, int64, ConfigStatus, ...socialhub.CallOption) error
	DeleteAdGroup(context.Context, int64, ...socialhub.CallOption) error
}

type CreativeWorkflow interface {
	ListCreatives(context.Context, int64, []ConfigStatus, ...socialhub.CallOption) ([]Creative, error)
	GetCreative(context.Context, int64, ...socialhub.CallOption) (*Creative, error)
	SetCreativeConfig(context.Context, int64, ConfigStatus, ...socialhub.CallOption) error
	DeleteCreative(context.Context, int64, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	AdAccountReport(context.Context, ReportRequest, ...socialhub.CallOption) (ReportResult, error)
	CampaignReport(context.Context, EntityReportRequest, ...socialhub.CallOption) (ReportResult, error)
	AdGroupReport(context.Context, EntityReportRequest, ...socialhub.CallOption) (ReportResult, error)
	CreativeReport(context.Context, EntityReportRequest, ...socialhub.CallOption) (ReportResult, error)
}

var (
	_ AccountWorkflow  = (*Client)(nil)
	_ CampaignWorkflow = (*Client)(nil)
	_ AdGroupWorkflow  = (*Client)(nil)
	_ CreativeWorkflow = (*Client)(nil)
	_ ReportWorkflow   = (*Client)(nil)
)

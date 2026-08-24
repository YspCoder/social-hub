package mailchimp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type CampaignType string

const (
	CampaignTypeRegular   CampaignType = "regular"
	CampaignTypePlaintext CampaignType = "plaintext"
	CampaignTypeABSplit   CampaignType = "absplit"
	CampaignTypeRSS       CampaignType = "rss"
	CampaignTypeVariate   CampaignType = "variate"
)

type CampaignStatus string

const (
	CampaignStatusSave      CampaignStatus = "save"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusSchedule  CampaignStatus = "schedule"
	CampaignStatusSending   CampaignStatus = "sending"
	CampaignStatusSent      CampaignStatus = "sent"
	CampaignStatusCanceled  CampaignStatus = "canceled"
	CampaignStatusCanceling CampaignStatus = "canceling"
	CampaignStatusArchived  CampaignStatus = "archived"
)

type SortDirection string

const (
	SortAscending  SortDirection = "ASC"
	SortDescending SortDirection = "DESC"
)

type ListSortField string

const ListSortDateCreated ListSortField = "date_created"

type CampaignSortField string

const (
	CampaignSortCreateTime CampaignSortField = "create_time"
	CampaignSortSendTime   CampaignSortField = "send_time"
)

type CampaignContentType string

const (
	CampaignContentTemplate     CampaignContentType = "template"
	CampaignContentHTML         CampaignContentType = "html"
	CampaignContentURL          CampaignContentType = "url"
	CampaignContentMultichannel CampaignContentType = "multichannel"
)

type DeliveryState string

const (
	DeliveryStateDelivering DeliveryState = "delivering"
	DeliveryStateDelivered  DeliveryState = "delivered"
	DeliveryStateCanceling  DeliveryState = "canceling"
	DeliveryStateCanceled   DeliveryState = "canceled"
)

// Pagination is Mailchimp's typed offset/count collection contract.
type Pagination struct {
	Count  int
	Offset int
}

type ListAudiencesRequest struct {
	Page Pagination
}

type GetAudienceRequest struct {
	AudienceID string
}

type ListListsRequest struct {
	Page                   Pagination
	SinceDateCreated       string
	BeforeDateCreated      string
	SinceCampaignLastSent  string
	BeforeCampaignLastSent string
	SortField              ListSortField
	SortDirection          SortDirection
	HasEcommerceStore      *bool
}

type GetListRequest struct {
	ListID string
}

type ListCampaignsRequest struct {
	Page             Pagination
	Type             CampaignType
	Status           CampaignStatus
	SinceSendTime    string
	BeforeSendTime   string
	SinceCreateTime  string
	BeforeCreateTime string
	ListID           string
	FolderID         string
	SortField        CampaignSortField
	SortDirection    SortDirection
}

type GetCampaignRequest struct {
	CampaignID string
}

type ListReportsRequest struct {
	Page           Pagination
	Type           CampaignType
	SinceSendTime  string
	BeforeSendTime string
}

type GetReportRequest struct {
	CampaignID string
}

type AudiencesWorkflow interface {
	ListAudiences(context.Context, ListAudiencesRequest, ...socialhub.CallOption) (AudiencePage, error)
	GetAudience(context.Context, GetAudienceRequest, ...socialhub.CallOption) (Audience, error)
}

type ListsWorkflow interface {
	ListLists(context.Context, ListListsRequest, ...socialhub.CallOption) (ListPage, error)
	GetList(context.Context, GetListRequest, ...socialhub.CallOption) (List, error)
}

type CampaignsWorkflow interface {
	ListCampaigns(context.Context, ListCampaignsRequest, ...socialhub.CallOption) (CampaignPage, error)
	GetCampaign(context.Context, GetCampaignRequest, ...socialhub.CallOption) (Campaign, error)
}

type ReportsWorkflow interface {
	ListReports(context.Context, ListReportsRequest, ...socialhub.CallOption) (ReportPage, error)
	GetReport(context.Context, GetReportRequest, ...socialhub.CallOption) (CampaignReport, error)
}

type ReadWorkflow interface {
	AudiencesWorkflow
	ListsWorkflow
	CampaignsWorkflow
	ReportsWorkflow
}

type ResponseMeta struct {
	RequestID          string
	RetryAfterHeader   string
	RetryAfter         time.Duration
	ConcurrencyLimit   int
	ConcurrencyLimited bool
	LimitHeaders       map[string]string
}

type AudienceStats struct {
	TotalContacts int `json:"total_contacts"`
}

type Audience struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Stats           AudienceStats   `json:"stats"`
	EnabledChannels []string        `json:"enabled_channels"`
	Meta            ResponseMeta    `json:"-"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Audience) UnmarshalJSON(data []byte) error {
	type wire Audience
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Audience(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type AudiencePage struct {
	Audiences  []Audience      `json:"audiences"`
	TotalItems int             `json:"total_items"`
	Page       Pagination      `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *AudiencePage) UnmarshalJSON(data []byte) error {
	type wire AudiencePage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = AudiencePage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ListStats struct {
	MemberCount      int     `json:"member_count"`
	UnsubscribeCount int     `json:"unsubscribe_count"`
	CleanedCount     int     `json:"cleaned_count"`
	CampaignCount    int     `json:"campaign_count"`
	CampaignLastSent string  `json:"campaign_last_sent"`
	OpenRate         float64 `json:"open_rate"`
	ClickRate        float64 `json:"click_rate"`
}

type List struct {
	ID                   string          `json:"id"`
	WebID                int             `json:"web_id"`
	Name                 string          `json:"name"`
	DateCreated          string          `json:"date_created"`
	ListRating           int             `json:"list_rating"`
	EmailTypeOption      bool            `json:"email_type_option"`
	Visibility           string          `json:"visibility"`
	DoubleOptIn          bool            `json:"double_optin"`
	HasWelcome           bool            `json:"has_welcome"`
	MarketingPermissions bool            `json:"marketing_permissions"`
	Stats                ListStats       `json:"stats"`
	Meta                 ResponseMeta    `json:"-"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *List) UnmarshalJSON(data []byte) error {
	type wire List
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = List(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ListPage struct {
	Lists      []List          `json:"lists"`
	TotalItems int             `json:"total_items"`
	Page       Pagination      `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *ListPage) UnmarshalJSON(data []byte) error {
	type wire ListPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ListPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CampaignRecipients struct {
	ListID         string `json:"list_id"`
	ListIsActive   bool   `json:"list_is_active"`
	ListName       string `json:"list_name"`
	RecipientCount int    `json:"recipient_count"`
}

type CampaignSettings struct {
	SubjectLine string `json:"subject_line"`
	PreviewText string `json:"preview_text"`
	Title       string `json:"title"`
	FolderID    string `json:"folder_id"`
}

type CampaignTracking struct {
	Opens        bool `json:"opens"`
	HTMLClicks   bool `json:"html_clicks"`
	TextClicks   bool `json:"text_clicks"`
	GoalTracking bool `json:"goal_tracking"`
	Ecommerce360 bool `json:"ecomm360"`
}

type EcommerceStats struct {
	TotalOrders  int     `json:"total_orders"`
	TotalSpent   float64 `json:"total_spent"`
	TotalRevenue float64 `json:"total_revenue"`
	CurrencyCode string  `json:"currency_code"`
}

type CampaignReportSummary struct {
	Opens            int            `json:"opens"`
	UniqueOpens      int            `json:"unique_opens"`
	OpenRate         float64        `json:"open_rate"`
	Clicks           int            `json:"clicks"`
	SubscriberClicks int            `json:"subscriber_clicks"`
	ClickRate        float64        `json:"click_rate"`
	Ecommerce        EcommerceStats `json:"ecommerce"`
}

type DeliveryStatus struct {
	Enabled        bool          `json:"enabled"`
	CanCancel      bool          `json:"can_cancel"`
	Status         DeliveryState `json:"status"`
	EmailsSent     int           `json:"emails_sent"`
	EmailsCanceled int           `json:"emails_canceled"`
}

type Campaign struct {
	ID               string                `json:"id"`
	WebID            int                   `json:"web_id"`
	ParentCampaignID string                `json:"parent_campaign_id"`
	Type             CampaignType          `json:"type"`
	CreateTime       string                `json:"create_time"`
	ArchiveURL       string                `json:"archive_url"`
	Status           CampaignStatus        `json:"status"`
	EmailsSent       int                   `json:"emails_sent"`
	SendTime         string                `json:"send_time"`
	ContentType      CampaignContentType   `json:"content_type"`
	Resendable       bool                  `json:"resendable"`
	Recipients       CampaignRecipients    `json:"recipients"`
	Settings         CampaignSettings      `json:"settings"`
	Tracking         CampaignTracking      `json:"tracking"`
	ReportSummary    CampaignReportSummary `json:"report_summary"`
	DeliveryStatus   DeliveryStatus        `json:"delivery_status"`
	Meta             ResponseMeta          `json:"-"`
	Raw              json.RawMessage       `json:"-"`
}

func (value *Campaign) UnmarshalJSON(data []byte) error {
	type wire Campaign
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Campaign(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CampaignPage struct {
	Campaigns  []Campaign      `json:"campaigns"`
	TotalItems int             `json:"total_items"`
	Page       Pagination      `json:"-"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

func (value *CampaignPage) UnmarshalJSON(data []byte) error {
	type wire CampaignPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CampaignPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type BounceStats struct {
	HardBounces  int `json:"hard_bounces"`
	SoftBounces  int `json:"soft_bounces"`
	SyntaxErrors int `json:"syntax_errors"`
}

type ForwardStats struct {
	ForwardsCount int `json:"forwards_count"`
	ForwardsOpens int `json:"forwards_opens"`
}

type OpenStats struct {
	OpensTotal               int     `json:"opens_total"`
	ProxyExcludedOpens       int     `json:"proxy_excluded_opens"`
	UniqueOpens              int     `json:"unique_opens"`
	ProxyExcludedUniqueOpens int     `json:"proxy_excluded_unique_opens"`
	OpenRate                 float64 `json:"open_rate"`
	ProxyExcludedOpenRate    float64 `json:"proxy_excluded_open_rate"`
	LastOpen                 string  `json:"last_open"`
}

type ClickStats struct {
	ClicksTotal            int     `json:"clicks_total"`
	UniqueClicks           int     `json:"unique_clicks"`
	UniqueSubscriberClicks int     `json:"unique_subscriber_clicks"`
	ClickRate              float64 `json:"click_rate"`
	LastClick              string  `json:"last_click"`
}

type IndustryStats struct {
	Type       string  `json:"type"`
	OpenRate   float64 `json:"open_rate"`
	ClickRate  float64 `json:"click_rate"`
	BounceRate float64 `json:"bounce_rate"`
	UnopenRate float64 `json:"unopen_rate"`
	UnsubRate  float64 `json:"unsub_rate"`
	AbuseRate  float64 `json:"abuse_rate"`
}

type ListPerformanceStats struct {
	SubscriptionRate      float64 `json:"sub_rate"`
	UnsubscriptionRate    float64 `json:"unsub_rate"`
	OpenRate              float64 `json:"open_rate"`
	ProxyExcludedOpenRate float64 `json:"proxy_excluded_open_rate"`
	ClickRate             float64 `json:"click_rate"`
}

type CampaignReport struct {
	ID             string               `json:"id"`
	CampaignTitle  string               `json:"campaign_title"`
	Type           string               `json:"type"`
	ListID         string               `json:"list_id"`
	ListIsActive   bool                 `json:"list_is_active"`
	ListName       string               `json:"list_name"`
	SubjectLine    string               `json:"subject_line"`
	PreviewText    string               `json:"preview_text"`
	EmailsSent     int                  `json:"emails_sent"`
	AbuseReports   int                  `json:"abuse_reports"`
	Unsubscribed   int                  `json:"unsubscribed"`
	SendTime       string               `json:"send_time"`
	RSSLastSend    string               `json:"rss_last_send"`
	Bounces        BounceStats          `json:"bounces"`
	Forwards       ForwardStats         `json:"forwards"`
	Opens          OpenStats            `json:"opens"`
	Clicks         ClickStats           `json:"clicks"`
	IndustryStats  IndustryStats        `json:"industry_stats"`
	ListStats      ListPerformanceStats `json:"list_stats"`
	Ecommerce      EcommerceStats       `json:"ecommerce"`
	DeliveryStatus DeliveryStatus       `json:"delivery_status"`
	Meta           ResponseMeta         `json:"-"`
	Raw            json.RawMessage      `json:"-"`
}

func (value *CampaignReport) UnmarshalJSON(data []byte) error {
	type wire CampaignReport
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CampaignReport(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ReportPage struct {
	Reports    []CampaignReport `json:"reports"`
	TotalItems int              `json:"total_items"`
	Page       Pagination       `json:"-"`
	Meta       ResponseMeta     `json:"-"`
	Raw        json.RawMessage  `json:"-"`
}

func (value *ReportPage) UnmarshalJSON(data []byte) error {
	type wire ReportPage
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ReportPage(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("mailchimp: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

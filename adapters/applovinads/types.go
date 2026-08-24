package applovinads

import (
	"encoding/json"
	"io"
)

type AccountType string

const (
	AccountTypeApp AccountType = "APP"
	AccountTypeWeb AccountType = "WEB"
)

type Status string

const (
	StatusLive   Status = "LIVE"
	StatusPaused Status = "PAUSED"
)

type AppPlatform string

const (
	PlatformIOS     AppPlatform = "IOS"
	PlatformAndroid AppPlatform = "ANDROID"
)

type BiddingStrategy string

const (
	BiddingTargetGoalCPI BiddingStrategy = "TARGET_GOAL_WITH_CPI_BILLING"
	BiddingAutoCPM       BiddingStrategy = "AUTO_BIDDING_WITH_CPM_BILLING"
)

type GoalType string

const (
	GoalCPI     GoalType = "CPI"
	GoalCPE     GoalType = "CPE"
	GoalCPP     GoalType = "CPP"
	GoalAdROAS  GoalType = "AD_ROAS"
	GoalIAPROAS GoalType = "CHK_ROAS"
	GoalMixROAS GoalType = "BLD_ROAS"
)

type ROASWindow string

const (
	ROASDay0  ROASWindow = "DAY0"
	ROASDay7  ROASWindow = "DAY7"
	ROASDay28 ROASWindow = "DAY28"
)

type TrackingMethod string

const (
	TrackingAdjust    TrackingMethod = "ADJUST"
	TrackingAppsFlyer TrackingMethod = "APPSFLYER"
	TrackingKochava   TrackingMethod = "KOCHAVA"
	TrackingBranch    TrackingMethod = "BRANCH"
	TrackingSingular  TrackingMethod = "SINGULAR"
	TrackingTenjin    TrackingMethod = "TENJIN"
)

type CatalogType string

const (
	CatalogDPA CatalogType = "DPA"
	CatalogDOA CatalogType = "DOA"
)

type AudienceStrategy string

const (
	AudienceUniversal   AudienceStrategy = "UNIVERSAL"
	AudienceProspecting AudienceStrategy = "PROSPECTING"
	AudienceDiscovery   AudienceStrategy = "DISCOVERY"
)

type Budget struct {
	DailyBudgetForAllCountries string            `json:"daily_budget_for_all_countries,omitempty"`
	CountryCodeToDailyBudget   map[string]string `json:"country_code_to_daily_budget,omitempty"`
}

type Goal struct {
	GoalType                 GoalType          `json:"goal_type,omitempty"`
	GoalValueForAllCountries string            `json:"goal_value_for_all_countries,omitempty"`
	CountryCodeToGoalValue   map[string]string `json:"country_code_to_goal_value,omitempty"`
	ROASDayTarget            ROASWindow        `json:"roas_day_target,omitempty"`
	EventTarget              string            `json:"event_target,omitempty"`
}

type GoalUpdate struct {
	GoalValueForAllCountries string            `json:"goal_value_for_all_countries,omitempty"`
	CountryCodeToGoalValue   map[string]string `json:"country_code_to_goal_value,omitempty"`
	ROASDayTarget            ROASWindow        `json:"roas_day_target,omitempty"`
	EventTarget              string            `json:"event_target,omitempty"`
}

type Targeting struct {
	CountryCode string   `json:"country_code"`
	RegionCodes []string `json:"region_codes,omitempty"`
	MetroNames  []string `json:"metro_names,omitempty"`
}

type Tracking struct {
	TrackingMethod TrackingMethod `json:"tracking_method"`
	ImpressionURL  string         `json:"impression_url"`
	ClickURL       string         `json:"click_url"`
}

type TrackingUpdate struct {
	ImpressionURL string `json:"impression_url,omitempty"`
	ClickURL      string `json:"click_url,omitempty"`
}

// Campaign is the lossless union of current APP and WEB Campaign fields.
type Campaign struct {
	ID                       string           `json:"id"`
	HashedID                 string           `json:"hashed_id,omitempty"`
	Name                     string           `json:"name"`
	Status                   Status           `json:"status,omitempty"`
	Type                     AccountType      `json:"type"`
	Platform                 AppPlatform      `json:"platform,omitempty"`
	BiddingStrategy          BiddingStrategy  `json:"bidding_strategy"`
	StartDate                string           `json:"start_date"`
	EndDate                  string           `json:"end_date,omitempty"`
	CreatedAt                string           `json:"created_at"`
	PackageName              string           `json:"package_name,omitempty"`
	ITunesID                 string           `json:"itunes_id,omitempty"`
	WebsiteURL               string           `json:"website_url,omitempty"`
	IsContinuousDelivery     *bool            `json:"is_continuous_delivery,omitempty"`
	IsCompositeBannerEnabled *bool            `json:"is_composite_banner_enabled,omitempty"`
	IsDynamicAdsEnabled      *bool            `json:"is_dynamic_ads_enabled,omitempty"`
	CatalogType              CatalogType      `json:"catalog_type,omitempty"`
	VariantSetID             string           `json:"variant_set_id,omitempty"`
	CatalogID                string           `json:"catalog_id,omitempty"`
	AudienceStrategy         AudienceStrategy `json:"audience_strategy,omitempty"`
	Budget                   Budget           `json:"budget"`
	Goal                     Goal             `json:"goal"`
	Tracking                 *Tracking        `json:"tracking,omitempty"`
	Targeting                []Targeting      `json:"targeting,omitempty"`
	Raw                      json.RawMessage  `json:"-"`
}

type CampaignRef struct {
	ID string `json:"id"`
}

type ListRequest struct {
	Page      int
	Size      int
	IDs       []string
	HashedIDs []string
}

type CampaignCreateRequest interface{ isCampaignCreateRequest() }

type AppCampaignCreateRequest struct {
	Name                     string
	StartDate                string
	EndDate                  string
	IsContinuousDelivery     *bool
	BiddingStrategy          BiddingStrategy
	Platform                 AppPlatform
	PackageName              string
	ITunesID                 string
	Budget                   Budget
	Goal                     Goal
	Targeting                []Targeting
	Tracking                 Tracking
	IsCompositeBannerEnabled *bool
}

func (AppCampaignCreateRequest) isCampaignCreateRequest() {}

type WebCampaignCreateRequest struct {
	Name                 string
	StartDate            string
	EndDate              string
	IsContinuousDelivery *bool
	BiddingStrategy      BiddingStrategy
	WebsiteURL           string
	Budget               Budget
	Goal                 Goal
	Targeting            []Targeting
	IsDynamicAdsEnabled  *bool
	CatalogType          CatalogType
	VariantSetID         string
	CatalogID            string
	AudienceStrategy     AudienceStrategy
}

func (WebCampaignCreateRequest) isCampaignCreateRequest() {}

type CampaignUpdateRequest interface{ isCampaignUpdateRequest() }

type AppCampaignUpdateRequest struct {
	ID                       string
	Name                     *string
	Status                   *Status
	EndDate                  *string
	IsContinuousDelivery     *bool
	Budget                   *Budget
	Goal                     *GoalUpdate
	Targeting                *[]Targeting
	Tracking                 *TrackingUpdate
	IsCompositeBannerEnabled *bool
}

func (AppCampaignUpdateRequest) isCampaignUpdateRequest() {}

type WebCampaignUpdateRequest struct {
	ID                   string
	Name                 *string
	Status               *Status
	EndDate              *string
	IsContinuousDelivery *bool
	Budget               *Budget
	Goal                 *GoalUpdate
	Targeting            *[]Targeting
	WebsiteURL           *string
	IsDynamicAdsEnabled  *bool
	CatalogType          *CatalogType
	VariantSetID         *string
	CatalogID            *string
	AudienceStrategy     *AudienceStrategy
}

func (WebCampaignUpdateRequest) isCampaignUpdateRequest() {}

type CatalogInfo struct {
	Catalogs []Catalog `json:"catalogs"`
}

type Catalog struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        CatalogType  `json:"type,omitempty"`
	VariantSets []VariantSet `json:"variant_sets"`
}

type VariantSet struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CatalogID     string `json:"catalog_id"`
	TotalVariants string `json:"total_variants,omitempty"`
	TotalItems    string `json:"total_items,omitempty"`
}

type CreativeAssetType string

const (
	AssetHostedHTML           CreativeAssetType = "HOSTED_HTML"
	AssetInterstitialPortrait CreativeAssetType = "IMG_INTER_P"
	AssetBanner               CreativeAssetType = "IMG_BANNER"
	AssetShortVideoPortrait   CreativeAssetType = "VID_SHORT_P"
	AssetLongVideoPortrait    CreativeAssetType = "VID_LONG_P"
)

type AssetRef struct {
	ID   string            `json:"id"`
	Type CreativeAssetType `json:"type,omitempty"`
}

type CreativeSetAsset struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Status       string            `json:"status,omitempty"`
	URL          string            `json:"url,omitempty"`
	Type         CreativeAssetType `json:"type,omitempty"`
	AssetType    CreativeAssetType `json:"asset_type,omitempty"`
	ResourceType string            `json:"resource_type,omitempty"`
}

type CreativeSet struct {
	ID             string             `json:"id"`
	HashedID       string             `json:"hashed_id,omitempty"`
	CampaignID     string             `json:"campaign_id,omitempty"`
	CampaignIDs    []string           `json:"campaign_ids,omitempty"`
	Type           AccountType        `json:"type"`
	Name           string             `json:"name"`
	Assets         []CreativeSetAsset `json:"assets"`
	Languages      []string           `json:"languages,omitempty"`
	Countries      []string           `json:"countries,omitempty"`
	ProductPage    string             `json:"product_page,omitempty"`
	CreativeSetURL string             `json:"creative_set_url,omitempty"`
	Status         Status             `json:"status,omitempty"`
	Version        string             `json:"version,omitempty"`
	CreatedAt      string             `json:"created_at,omitempty"`
	Raw            json.RawMessage    `json:"-"`
}

type CreativeSetRef struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

type CreativeSetCreateRequest interface{ isCreativeSetCreateRequest() }

type AppCreativeSetCreateRequest struct {
	CampaignID  string
	Name        string
	Assets      []AssetRef
	Languages   []string
	Countries   []string
	ProductPage string
}

func (AppCreativeSetCreateRequest) isCreativeSetCreateRequest() {}

type WebCreativeSetCreateRequest struct {
	CampaignID     string
	Name           string
	Assets         []AssetRef
	Languages      []string
	Countries      []string
	CreativeSetURL string
}

func (WebCreativeSetCreateRequest) isCreativeSetCreateRequest() {}

type CreativeSetUpdateRequest interface{ isCreativeSetUpdateRequest() }

type AppCreativeSetUpdateRequest struct {
	ID          string
	CampaignID  string
	Name        *string
	Assets      *[]AssetRef
	Languages   *[]string
	Countries   *[]string
	ProductPage *string
	Status      *Status
}

func (AppCreativeSetUpdateRequest) isCreativeSetUpdateRequest() {}

type WebCreativeSetUpdateRequest struct {
	ID             string
	CampaignID     string
	Name           *string
	Assets         *[]AssetRef
	Languages      *[]string
	Countries      *[]string
	CreativeSetURL *string
	Status         *Status
}

func (WebCreativeSetUpdateRequest) isCreativeSetUpdateRequest() {}

type CreativeSetsByCampaign struct {
	CampaignCount    int64                    `json:"campaign_count"`
	CreativeSetCount int64                    `json:"creative_set_count"`
	Campaigns        map[string][]CreativeSet `json:"campaigns"`
}

type CloneCreativeSetRequest struct {
	CampaignID    string
	CreativeSetID string
}

type CreativeSetAssociationRequest struct {
	CampaignIDs    []int64 `json:"campaign_ids"`
	CreativeSetIDs []int64 `json:"creative_set_ids"`
}

type CreativeSetRemovalRequest struct {
	CampaignIDs   []int64 `json:"campaign_ids"`
	CreativeSetID int64   `json:"creative_set_id"`
}

type ResourceType string

const (
	ResourceImage ResourceType = "image"
	ResourceHTML  ResourceType = "html"
	ResourceVideo ResourceType = "video"
)

type Asset struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Status           string            `json:"status,omitempty"`
	URL              string            `json:"url"`
	AssetType        CreativeAssetType `json:"asset_type"`
	ResourceType     string            `json:"resource_type"`
	UploadTime       string            `json:"upload_time,omitempty"`
	ViolationReasons []string          `json:"violation_reasons,omitempty"`
}

type ListAssetsRequest struct {
	Page         int
	Size         int
	IDs          []string
	ResourceType ResourceType
}

type UploadFile struct {
	Filename    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

type UploadRef struct {
	UploadID string `json:"upload_id"`
}

type UploadStatus struct {
	Summary      UploadSummary  `json:"summary"`
	Details      []UploadDetail `json:"details"`
	UploadStatus string         `json:"upload_status"`
}

type UploadSummary struct {
	Total   int64 `json:"total"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Pending int64 `json:"pending"`
}

type UploadDetail struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	UploadTime   string `json:"uploadTime,omitempty"`
	URL          string `json:"url,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	FileStatus   string `json:"file_status"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type AssetAssociationRequest struct {
	AssetIDs       []int64 `json:"asset_ids"`
	CreativeSetIDs []int64 `json:"creative_set_ids"`
}

type AssetRemovalRequest struct {
	AssetID        int64   `json:"asset_id"`
	CreativeSetIDs []int64 `json:"creative_set_ids"`
}

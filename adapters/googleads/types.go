package googleads

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

// CustomerWorkflow exposes account identity and OAuth-accessible customers.
type CustomerWorkflow interface {
	GetCustomer(context.Context, ...socialhub.CallOption) (*Customer, error)
	ListAccessibleCustomers(context.Context, ...socialhub.CallOption) ([]string, error)
}

// BudgetWorkflow manages Campaign Budget resources.
type BudgetWorkflow interface {
	ListCampaignBudgets(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[CampaignBudget], error)
	CreateCampaignBudget(context.Context, CreateCampaignBudgetRequest, ...socialhub.CallOption) (*CampaignBudget, error)
	UpdateCampaignBudget(context.Context, string, UpdateCampaignBudgetRequest, ...socialhub.CallOption) (*CampaignBudget, error)
	RemoveCampaignBudget(context.Context, string, ...socialhub.CallOption) error
}

// CampaignWorkflow manages Search Campaign resources. Creates are always
// PAUSED; enabling spend is a separate explicit call.
type CampaignWorkflow interface {
	ListCampaigns(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[Campaign], error)
	CreateCampaign(context.Context, CreateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	UpdateCampaign(context.Context, string, UpdateCampaignRequest, ...socialhub.CallOption) (*Campaign, error)
	SetCampaignStatus(context.Context, string, Status, ...socialhub.CallOption) (*Campaign, error)
	RemoveCampaign(context.Context, string, ...socialhub.CallOption) error
}

// AdGroupWorkflow manages Search Ad Group resources. Creates are always
// PAUSED; enabling spend is a separate explicit call.
type AdGroupWorkflow interface {
	ListAdGroups(context.Context, ListAdGroupsRequest, ...socialhub.CallOption) (TokenPage[AdGroup], error)
	CreateAdGroup(context.Context, CreateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	UpdateAdGroup(context.Context, string, UpdateAdGroupRequest, ...socialhub.CallOption) (*AdGroup, error)
	SetAdGroupStatus(context.Context, string, Status, ...socialhub.CallOption) (*AdGroup, error)
	RemoveAdGroup(context.Context, string, ...socialhub.CallOption) error
}

// AdWorkflow manages responsive search ads. Ad content is updated through the
// Ad service; status and removal use the containing AdGroupAd resource.
type AdWorkflow interface {
	ListResponsiveSearchAds(context.Context, ListAdsRequest, ...socialhub.CallOption) (TokenPage[AdGroupAd], error)
	CreateResponsiveSearchAd(context.Context, CreateResponsiveSearchAdRequest, ...socialhub.CallOption) (*AdGroupAd, error)
	UpdateResponsiveSearchAd(context.Context, string, UpdateResponsiveSearchAdRequest, ...socialhub.CallOption) (*Ad, error)
	SetAdStatus(context.Context, string, Status, ...socialhub.CallOption) (*AdGroupAd, error)
	RemoveAd(context.Context, string, ...socialhub.CallOption) error
}

// ReportWorkflow exposes bounded, read-only GAQL Search pagination.
type ReportWorkflow interface {
	Search(context.Context, SearchRequest, ...socialhub.CallOption) (SearchPage, error)
}

type TokenPage[T any] struct {
	Items         []T
	NextPageToken string
}

type ListRequest struct {
	PageToken string
}

type ListAdGroupsRequest struct {
	CampaignResourceName string
	PageToken            string
}

type ListAdsRequest struct {
	AdGroupResourceName string
	PageToken           string
}

type Status string

const (
	StatusEnabled Status = "ENABLED"
	StatusPaused  Status = "PAUSED"
)

type EUPoliticalAdvertising string

const (
	ContainsEUPoliticalAdvertising       EUPoliticalAdvertising = "CONTAINS_EU_POLITICAL_ADVERTISING"
	DoesNotContainEUPoliticalAdvertising EUPoliticalAdvertising = "DOES_NOT_CONTAIN_EU_POLITICAL_ADVERTISING"
)

type Customer struct {
	ResourceName    string          `json:"resourceName"`
	ID              string          `json:"id,omitempty"`
	DescriptiveName string          `json:"descriptiveName,omitempty"`
	CurrencyCode    string          `json:"currencyCode,omitempty"`
	TimeZone        string          `json:"timeZone,omitempty"`
	Manager         bool            `json:"manager,omitempty"`
	TestAccount     bool            `json:"testAccount,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

func (value *Customer) UnmarshalJSON(data []byte) error {
	type alias Customer
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Customer(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CampaignBudget struct {
	ResourceName     string          `json:"resourceName"`
	ID               string          `json:"id,omitempty"`
	Name             string          `json:"name,omitempty"`
	AmountMicros     string          `json:"amountMicros,omitempty"`
	DeliveryMethod   string          `json:"deliveryMethod,omitempty"`
	Period           string          `json:"period,omitempty"`
	ExplicitlyShared bool            `json:"explicitlyShared,omitempty"`
	ReferenceCount   string          `json:"referenceCount,omitempty"`
	Status           string          `json:"status,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

func (value *CampaignBudget) UnmarshalJSON(data []byte) error {
	type alias CampaignBudget
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = CampaignBudget(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateCampaignBudgetRequest struct {
	Name             string
	AmountMicros     int64
	ExplicitlyShared *bool
	Fields           map[string]any
}

type UpdateCampaignBudgetRequest struct {
	Name         *string
	AmountMicros *int64
}

type Campaign struct {
	ResourceName                   string                 `json:"resourceName"`
	ID                             string                 `json:"id,omitempty"`
	Name                           string                 `json:"name,omitempty"`
	Status                         Status                 `json:"status,omitempty"`
	AdvertisingChannelType         string                 `json:"advertisingChannelType,omitempty"`
	CampaignBudget                 string                 `json:"campaignBudget,omitempty"`
	ContainsEUPoliticalAdvertising EUPoliticalAdvertising `json:"containsEuPoliticalAdvertising,omitempty"`
	ServingStatus                  string                 `json:"servingStatus,omitempty"`
	PrimaryStatus                  string                 `json:"primaryStatus,omitempty"`
	Raw                            json.RawMessage        `json:"-"`
}

func (value *Campaign) UnmarshalJSON(data []byte) error {
	type alias Campaign
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Campaign(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type NetworkSettings struct {
	TargetGoogleSearch   bool `json:"targetGoogleSearch"`
	TargetSearchNetwork  bool `json:"targetSearchNetwork"`
	TargetContentNetwork bool `json:"targetContentNetwork"`
}

type CreateCampaignRequest struct {
	Name                           string
	BudgetResourceName             string
	ContainsEUPoliticalAdvertising EUPoliticalAdvertising
	NetworkSettings                *NetworkSettings
	Fields                         map[string]any
}

type UpdateCampaignRequest struct {
	Name               *string
	BudgetResourceName *string
	NetworkSettings    *NetworkSettings
}

type AdGroup struct {
	ResourceName  string          `json:"resourceName"`
	ID            string          `json:"id,omitempty"`
	Campaign      string          `json:"campaign,omitempty"`
	Name          string          `json:"name,omitempty"`
	Status        Status          `json:"status,omitempty"`
	Type          string          `json:"type,omitempty"`
	CPCBidMicros  string          `json:"cpcBidMicros,omitempty"`
	PrimaryStatus string          `json:"primaryStatus,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

func (value *AdGroup) UnmarshalJSON(data []byte) error {
	type alias AdGroup
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdGroup(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type CreateAdGroupRequest struct {
	CampaignResourceName string
	Name                 string
	CPCBidMicros         int64
	Fields               map[string]any
}

type UpdateAdGroupRequest struct {
	Name         *string
	CPCBidMicros *int64
}

type AdGroupAd struct {
	ResourceName  string          `json:"resourceName"`
	AdGroup       string          `json:"adGroup,omitempty"`
	Status        Status          `json:"status,omitempty"`
	PrimaryStatus string          `json:"primaryStatus,omitempty"`
	AdStrength    string          `json:"adStrength,omitempty"`
	Ad            Ad              `json:"ad"`
	Raw           json.RawMessage `json:"-"`
}

func (value *AdGroupAd) UnmarshalJSON(data []byte) error {
	type alias AdGroupAd
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AdGroupAd(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Ad struct {
	ResourceName       string                  `json:"resourceName"`
	ID                 string                  `json:"id,omitempty"`
	Name               string                  `json:"name,omitempty"`
	Type               string                  `json:"type,omitempty"`
	FinalURLs          []string                `json:"finalUrls,omitempty"`
	ResponsiveSearchAd *ResponsiveSearchAdInfo `json:"responsiveSearchAd,omitempty"`
	Raw                json.RawMessage         `json:"-"`
}

func (value *Ad) UnmarshalJSON(data []byte) error {
	type alias Ad
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Ad(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ResponsiveSearchAdInfo struct {
	Headlines    []AdTextAsset `json:"headlines,omitempty"`
	Descriptions []AdTextAsset `json:"descriptions,omitempty"`
	Path1        string        `json:"path1,omitempty"`
	Path2        string        `json:"path2,omitempty"`
}

type AdTextAsset struct {
	Text                  string `json:"text"`
	PinnedField           string `json:"pinnedField,omitempty"`
	AssetPerformanceLabel string `json:"assetPerformanceLabel,omitempty"`
}

type CreateResponsiveSearchAdRequest struct {
	AdGroupResourceName string
	Name                string
	FinalURLs           []string
	Headlines           []AdTextAsset
	Descriptions        []AdTextAsset
	Path1               string
	Path2               string
	Fields              map[string]any
}

type UpdateResponsiveSearchAdRequest struct {
	Name         *string
	FinalURLs    *[]string
	Headlines    *[]AdTextAsset
	Descriptions *[]AdTextAsset
	Path1        *string
	Path2        *string
}

type SearchRequest struct {
	Query        string
	PageToken    string
	ValidateOnly bool
}

type SearchPage struct {
	Rows                     []json.RawMessage
	NextPageToken            string
	FieldMask                string
	TotalResultsCount        string
	QueryResourceConsumption string
}

type searchResponse[T any] struct {
	Results                  []T    `json:"results"`
	NextPageToken            string `json:"nextPageToken"`
	FieldMask                string `json:"fieldMask"`
	TotalResultsCount        string `json:"totalResultsCount"`
	QueryResourceConsumption string `json:"queryResourceConsumption"`
}

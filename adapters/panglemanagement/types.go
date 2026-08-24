package panglemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"social-hub/pkg/socialhub"
)

type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var value string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
	} else {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		decoder.UseNumber()
		var number json.Number
		if err := decoder.Decode(&number); err != nil {
			return fmt.Errorf("panglemanagement: identifier must be a decimal string or number")
		}
		value = number.String()
	}
	if !validNumericID(value) {
		return fmt.Errorf("panglemanagement: invalid identifier")
	}
	*id = ID(value)
	return nil
}

func (id ID) MarshalJSON() ([]byte, error) {
	if !validNumericID(string(id)) {
		return nil, fmt.Errorf("panglemanagement: invalid identifier")
	}
	return []byte(id), nil
}

func IDList(values ...ID) *[]ID {
	copy := append([]ID(nil), values...)
	return &copy
}

type Decimal string

func (decimal *Decimal) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var value string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
	} else {
		value = string(trimmed)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || value == "" {
		return fmt.Errorf("panglemanagement: invalid decimal")
	}
	*decimal = Decimal(value)
	return nil
}

func (decimal Decimal) String() string { return string(decimal) }

type Money string
type Currency string
type Region string

const (
	CurrencyUSD Currency = "usd"
	CurrencyCNY Currency = "cny"
)

type AppStatus int

const (
	AppStatusResume    AppStatus = -1
	AppStatusReview    AppStatus = 1
	AppStatusLive      AppStatus = 2
	AppStatusRejected  AppStatus = 3
	AppStatusSuspended AppStatus = 4
	AppStatusAborted   AppStatus = 5
	AppStatusTest      AppStatus = 6
)

type PlacementStatus int

const (
	PlacementStatusResume PlacementStatus = -1
	PlacementStatusLive   PlacementStatus = 2
	PlacementStatusPaused PlacementStatus = 3
	PlacementStatusTest   PlacementStatus = 6
)

type OSType string

const (
	OSIOS     OSType = "ios"
	OSAndroid OSType = "android"
)

type COPPA int

const (
	COPPAClientConfigured COPPA = -1
	COPPAOver12           COPPA = 0
	COPPAUnder13          COPPA = 1
)

type App struct {
	ID              ID        `json:"app_id"`
	UserID          ID        `json:"user_id"`
	Status          AppStatus `json:"status"`
	CategoryCode    int       `json:"app_category_code"`
	Name            string    `json:"app_name"`
	PackageName     string    `json:"package_name"`
	OS              OSType    `json:"os_type"`
	DownloadURL     string    `json:"download_url"`
	DownloadAddress string    `json:"download_address,omitempty"`
	APKSign         string    `json:"apk_sign,omitempty"`
	DebugAPKSign    string    `json:"debug_apk_sign,omitempty"`
	MaskRuleIDs     []ID      `json:"mask_rule_ids"`
	COPPA           *COPPA    `json:"coppa_value,omitempty"`
}

type CreateAppRequest struct {
	Status       AppStatus
	CategoryCode int
	Name         string
	DownloadURL  string
	MaskRuleID   *int64
	MaskRuleIDs  *[]ID
	COPPA        *COPPA
}

type UpdateAppRequest struct {
	AppID        ID
	Status       *AppStatus
	CategoryCode *int
	Name         *string
	DownloadURL  *string
	MaskRuleIDs  *[]ID
	COPPA        *COPPA
}

type ListAppsRequest struct {
	Page     int
	PageSize int
	IDs      []ID
	Names    []string
	OS       []OSType
	Statuses []AppStatus
}

type PageInfo struct {
	Page        int `json:"page"`
	PageSize    int `json:"page_size"`
	TotalNumber int `json:"total_number"`
	TotalPages  int `json:"total_page"`
}

type AppPage struct {
	Apps     []App
	PageInfo PageInfo
	HasMore  bool
}

type AppMutationResult struct {
	AppID         ID
	Status        AppStatus
	PendingReview bool
	RequestID     string
}

type AdSlotType int

const (
	AdSlotNative        AdSlotType = 1
	AdSlotBanner        AdSlotType = 2
	AdSlotAppOpen       AdSlotType = 3
	AdSlotRewardedVideo AdSlotType = 5
	AdSlotInterstitial  AdSlotType = 6
)

type BiddingType int

const (
	BiddingFixedCPM   BiddingType = 0
	BiddingInApp      BiddingType = 1
	BiddingClientSide BiddingType = 2
)

type Orientation int

const (
	OrientationVertical   Orientation = 1
	OrientationHorizontal Orientation = 2
)

type AcceptMaterialType int

const (
	AcceptImageOnly     AcceptMaterialType = 1
	AcceptVideoOnly     AcceptMaterialType = 2
	AcceptVideoAndImage AcceptMaterialType = 3
)

type AdCategory int

const (
	AdCategoryVideo       AdCategory = 4
	AdCategoryWideImage   AdCategory = 11
	AdCategorySquareImage AdCategory = 12
	AdCategorySquareVideo AdCategory = 13
)

type SlideBanner int

const (
	SlideBannerDisabled SlideBanner = 1
	SlideBannerEnabled  SlideBanner = 2
)

type PlacementSpec interface{ placementSpec() }

type NativeSpec struct{ Categories []AdCategory }

func (NativeSpec) placementSpec() {}

type BannerSpec struct {
	Slide  SlideBanner
	Width  int
	Height int
}

func (BannerSpec) placementSpec() {}

type AppOpenSpec struct {
	Orientation    Orientation
	AcceptMaterial *AcceptMaterialType
}

func (AppOpenSpec) placementSpec() {}

type RewardedVideoSpec struct {
	Orientation  Orientation
	RewardName   string
	RewardCount  int64
	VerifyServer bool
	CallbackURL  string
}

func (RewardedVideoSpec) placementSpec() {}

type InterstitialSpec struct {
	Orientation    Orientation
	AcceptMaterial *AcceptMaterialType
}

func (InterstitialSpec) placementSpec() {}

type CreatePlacementRequest struct {
	AppID       ID
	Name        string
	MaskRuleID  *int64
	MaskRuleIDs *[]ID
	Bidding     *BiddingType
	CPM         Money
	Currency    Currency
	CPMByRegion map[Region]Money
	Spec        PlacementSpec
}

type UpdatePlacementRequest struct {
	AdSlotID          ID
	Name              *string
	Status            *PlacementStatus
	MaskRuleIDs       *[]ID
	CPM               *Money
	Currency          *Currency
	CPMByRegion       map[Region]Money
	Categories        *[]AdCategory
	SlideBanner       *SlideBanner
	Orientation       *Orientation
	RewardName        *string
	RewardCount       *int64
	RewardIsCallback  *bool
	RewardCallbackURL *string
	UpdateSecurityKey *bool
	AcceptMaterial    *AcceptMaterialType
}

type ListPlacementsRequest struct {
	Page     int
	PageSize int
	IDs      []ID
	Names    []string
	AppIDs   []ID
	AppNames []string
	Types    []AdSlotType
	Statuses []PlacementStatus
}

type ExpectedCPM struct {
	Country Region  `json:"country"`
	CPM     Decimal `json:"expected_cpm"`
}

type Placement struct {
	ID                ID                 `json:"ad_slot_id"`
	Name              string             `json:"ad_slot_name"`
	AppID             ID                 `json:"app_id"`
	AppName           string             `json:"app_name"`
	Status            PlacementStatus    `json:"status"`
	Type              AdSlotType         `json:"ad_slot_type"`
	RenderType        int                `json:"render_type"`
	MaskRuleID        ID                 `json:"mask_rule_id,omitempty"`
	MaskRuleIDs       []ID               `json:"mask_rule_ids"`
	BiddingType       BiddingType        `json:"bidding_type"`
	Categories        []AdCategory       `json:"ad_categories,omitempty"`
	Width             int                `json:"width,omitempty"`
	Height            int                `json:"height,omitempty"`
	Orientation       Orientation        `json:"orientation,omitempty"`
	RewardName        string             `json:"reward_name,omitempty"`
	RewardCount       int64              `json:"reward_count,omitempty"`
	RewardIsCallback  int                `json:"reward_is_callback,omitempty"`
	RewardCallbackURL string             `json:"reward_callback_url,omitempty"`
	RewardSecurityKey string             `json:"reward_security_key,omitempty"`
	CPM               Decimal            `json:"cpm,omitempty"`
	ExpectedCPM       []ExpectedCPM      `json:"expected_cpm_list,omitempty"`
	UseMediation      int                `json:"use_mediation"`
	AcceptMaterial    AcceptMaterialType `json:"accept_material_type,omitempty"`
}

type PlacementPage struct {
	Placements []Placement
	PageInfo   PageInfo
	HasMore    bool
}

type PlacementMutationResult struct {
	AdSlotID          ID
	Status            PlacementStatus
	RewardSecurityKey string
	RequestID         string
}

type UpdateExpectedCPMRequest struct {
	AdSlotID ID
	AppID    ID
	CPM      Money
	Currency Currency
	Delete   bool
}

type MutationReceipt struct{ RequestID string }

type AppsWorkflow interface {
	CreateApp(context.Context, CreateAppRequest, ...socialhub.CallOption) (AppMutationResult, error)
	UpdateApp(context.Context, UpdateAppRequest, ...socialhub.CallOption) (AppMutationResult, error)
	ListApps(context.Context, ListAppsRequest, ...socialhub.CallOption) (AppPage, error)
}

type PlacementsWorkflow interface {
	CreatePlacement(context.Context, CreatePlacementRequest, ...socialhub.CallOption) (PlacementMutationResult, error)
	UpdatePlacement(context.Context, UpdatePlacementRequest, ...socialhub.CallOption) (PlacementMutationResult, error)
	ListPlacements(context.Context, ListPlacementsRequest, ...socialhub.CallOption) (PlacementPage, error)
	UpdateExpectedCPM(context.Context, UpdateExpectedCPMRequest, ...socialhub.CallOption) (MutationReceipt, error)
}

var _ AppsWorkflow = (*Client)(nil)
var _ PlacementsWorkflow = (*Client)(nil)

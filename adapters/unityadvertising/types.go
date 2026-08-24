package unityadvertising

import (
	"encoding/json"
	"strconv"
)

// Page is Unity's list response envelope. Some endpoints omit Offset and Limit.
type Page[T any] struct {
	Total   int64 `json:"total"`
	Offset  int64 `json:"offset,omitempty"`
	Limit   int   `json:"limit,omitempty"`
	Results []T   `json:"results"`
}

// NullableString distinguishes an omitted field from an explicit JSON null.
// Use a nil *NullableString to omit a field.
type NullableString struct {
	Value *string
}

func NewNullableString(value string) *NullableString { return &NullableString{Value: &value} }
func NewNullString() *NullableString                 { return &NullableString{} }

func (value NullableString) MarshalJSON() ([]byte, error) { return json.Marshal(value.Value) }

type Store string

const (
	StoreApple             Store = "apple"
	StoreGoogle            Store = "google"
	StoreStandaloneAndroid Store = "standalone_android"
)

type CountryCode string
type Money string
type BidAmount string
type ROASGoal string
type EventOptimizationType string
type SDKEventName string

type AttributionUpdateType string

const (
	AttributionNewAudiences            AttributionUpdateType = "newAudiences"
	AttributionNewAndExistingLive      AttributionUpdateType = "newAndExistingLiveAudiences"
	AttributionNewAndExistingAudiences AttributionUpdateType = "newAndExistingAudiences"
)

type CreativeLanguage string

const (
	LanguageNone         CreativeLanguage = "zxx"
	LanguageUndetermined CreativeLanguage = "und"
	LanguageEnglish      CreativeLanguage = "en"
	LanguageChinese      CreativeLanguage = "zh"
	LanguageFrench       CreativeLanguage = "fr"
	LanguageGerman       CreativeLanguage = "de"
	LanguageJapanese     CreativeLanguage = "ja"
	LanguageKorean       CreativeLanguage = "ko"
	LanguageSpanish      CreativeLanguage = "es"
)

type CreativeType string

const (
	CreativeSquareEndCard      CreativeType = "squareEndCard"
	CreativeEndCardPair        CreativeType = "endCardPair"
	CreativePortraitVideo      CreativeType = "portraitVideo"
	CreativeLandscapeVideo     CreativeType = "landscapeVideo"
	CreativeSquareVideo        CreativeType = "squareVideo"
	CreativeResponsivePlayable CreativeType = "responsivePlayable"
	CreativePortraitPlayable   CreativeType = "portraitPlayable"
	CreativeLandscapePlayable  CreativeType = "landscapePlayable"
)

type CreativeStatus string

const (
	CreativeUploaded          CreativeStatus = "uploaded"
	CreativeProcessing        CreativeStatus = "processing"
	CreativeProcessingFailed  CreativeStatus = "processingFailed"
	CreativePendingModeration CreativeStatus = "pendingModeration"
	CreativeApproved          CreativeStatus = "approved"
	CreativeRejected          CreativeStatus = "rejected"
)

type CreativePackType string

const (
	CreativePackVideo         CreativePackType = "video"
	CreativePackPlayable      CreativePackType = "playable"
	CreativePackVideoPlayable CreativePackType = "video+playable"
)

type CampaignGoal string

const (
	CampaignGoalInstalls          CampaignGoal = "installs"
	CampaignGoalRetention         CampaignGoal = "retention"
	CampaignGoalROAS              CampaignGoal = "roas"
	CampaignGoalCreativeTesting   CampaignGoal = "creativeTesting"
	CampaignGoalEventOptimization CampaignGoal = "eventOptimization"
)

type BillingType string

const (
	BillingCPI BillingType = "cpi"
	BillingCPM BillingType = "cpm"
)

type BiddingStrategy string

const (
	BiddingManual    BiddingStrategy = "manual"
	BiddingAutomated BiddingStrategy = "automated"
)

type CampaignStatus string

const (
	CampaignStatusLive     CampaignStatus = "live"
	CampaignStatusLearning CampaignStatus = "learning"
	CampaignStatusPaused   CampaignStatus = "paused"
)

type AutoStart string

const (
	AutoStartEnabled  AutoStart = "enabled"
	AutoStartDisabled AutoStart = "disabled"
	AutoStartCanceled AutoStart = "canceled"
)

type ROASType string

const (
	ROASTypeIAP       ROASType = "iap"
	ROASTypeAdRevenue ROASType = "adRevenue"
)

type PostInstallWindow string

const (
	PostInstallD0  PostInstallWindow = "d0"
	PostInstallD3  PostInstallWindow = "d3"
	PostInstallD7  PostInstallWindow = "d7"
	PostInstallD28 PostInstallWindow = "d28"
)

type CampaignIncludeField string

const (
	IncludeCPIBids               CampaignIncludeField = "cpiBids"
	IncludeSourceBids            CampaignIncludeField = "sourceBids"
	IncludeROASBids              CampaignIncludeField = "roasBids"
	IncludeRetentionBids         CampaignIncludeField = "retentionBids"
	IncludeEventOptimizationBids CampaignIncludeField = "eventOptimizationBids"
	IncludeBudget                CampaignIncludeField = "budget"
)

func captureRaw[T any](data []byte, target *T, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func validPage[T any](page Page[T], maximumLimit int, valid func(T) bool) bool {
	if page.Total < 0 || page.Offset < 0 || page.Limit < 0 || page.Limit > maximumLimit || int64(len(page.Results)) > page.Total {
		return false
	}
	for _, item := range page.Results {
		if !valid(item) {
			return false
		}
	}
	return true
}

func formatInt(value int) string     { return strconv.Itoa(value) }
func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }
func formatBool(value bool) string   { return strconv.FormatBool(value) }

func digitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

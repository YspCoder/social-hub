package merchantapi

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

type AccountWorkflow interface {
	GetMerchantAccount(context.Context, ...socialhub.CallOption) (*MerchantAccount, error)
	ListAccountIssues(context.Context, IssueListRequest, ...socialhub.CallOption) (TokenPage[AccountIssue], error)
}

type DataSourceWorkflow interface {
	ListDataSources(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[DataSource], error)
	GetDataSource(context.Context, string, ...socialhub.CallOption) (*DataSource, error)
}

type ProductWorkflow interface {
	ListProducts(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[Product], error)
	GetProduct(context.Context, string, ...socialhub.CallOption) (*Product, error)
	InsertProductInput(context.Context, InsertProductInputRequest, ...socialhub.CallOption) (*ProductInput, error)
	PatchProductInput(context.Context, PatchProductInputRequest, ...socialhub.CallOption) (*ProductInput, error)
	DeleteProductInput(context.Context, DeleteProductInputRequest, ...socialhub.CallOption) error
}

type ReportWorkflow interface {
	SearchReports(context.Context, ReportRequest, ...socialhub.CallOption) (ReportPage, error)
}

type QuotaWorkflow interface {
	ListQuotaGroups(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[QuotaGroup], error)
	ListProductLimits(context.Context, ListRequest, ...socialhub.CallOption) (TokenPage[AccountLimit], error)
}

type TokenPage[T any] struct {
	Items         []T
	NextPageToken string
}

type ListRequest struct {
	PageSize  int
	PageToken string
}

type IssueListRequest struct {
	PageSize     int
	PageToken    string
	LanguageCode string
	TimeZone     string
}

type TimeZone struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

type MerchantAccount struct {
	Name         string          `json:"name"`
	AccountID    string          `json:"accountId"`
	AccountName  string          `json:"accountName"`
	TimeZone     TimeZone        `json:"timeZone"`
	LanguageCode string          `json:"languageCode"`
	AdultContent bool            `json:"adultContent"`
	TestAccount  bool            `json:"testAccount"`
	Raw          json.RawMessage `json:"-"`
}

func (value *MerchantAccount) UnmarshalJSON(data []byte) error {
	type alias MerchantAccount
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = MerchantAccount(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type IssueSeverity string

const (
	IssueCritical   IssueSeverity = "CRITICAL"
	IssueError      IssueSeverity = "ERROR"
	IssueSuggestion IssueSeverity = "SUGGESTION"
)

type ReportingContext string

const (
	ContextShoppingAds       ReportingContext = "SHOPPING_ADS"
	ContextDemandGenAds      ReportingContext = "DEMAND_GEN_ADS"
	ContextDisplayAds        ReportingContext = "DISPLAY_ADS"
	ContextLocalInventoryAds ReportingContext = "LOCAL_INVENTORY_ADS"
	ContextVehicleAds        ReportingContext = "VEHICLE_INVENTORY_ADS"
	ContextFreeListings      ReportingContext = "FREE_LISTINGS"
	ContextYouTubeShopping   ReportingContext = "YOUTUBE_SHOPPING"
)

type ImpactedDestination struct {
	ReportingContext ReportingContext  `json:"reportingContext"`
	Impacts          []json.RawMessage `json:"impacts"`
}

type AccountIssue struct {
	Name                 string                `json:"name"`
	Title                string                `json:"title"`
	Detail               string                `json:"detail"`
	Severity             IssueSeverity         `json:"severity"`
	DocumentationURI     string                `json:"documentationUri"`
	ImpactedDestinations []ImpactedDestination `json:"impactedDestinations"`
	Raw                  json.RawMessage       `json:"-"`
}

func (value *AccountIssue) UnmarshalJSON(data []byte) error {
	type alias AccountIssue
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AccountIssue(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type DataSourceInput string

const (
	DataSourceAPI      DataSourceInput = "API"
	DataSourceFile     DataSourceInput = "FILE"
	DataSourceUI       DataSourceInput = "UI"
	DataSourceAutofeed DataSourceInput = "AUTOFEED"
)

type PrimaryProductDataSource struct {
	ContentLanguage string            `json:"contentLanguage,omitempty"`
	FeedLabel       string            `json:"feedLabel,omitempty"`
	LegacyLocal     bool              `json:"legacyLocal,omitempty"`
	Countries       []string          `json:"countries,omitempty"`
	Destinations    []json.RawMessage `json:"destinations,omitempty"`
	DefaultRule     json.RawMessage   `json:"defaultRule,omitempty"`
}

type DataSource struct {
	Name                          string                    `json:"name"`
	DataSourceID                  string                    `json:"dataSourceId"`
	DisplayName                   string                    `json:"displayName"`
	Input                         DataSourceInput           `json:"input"`
	PrimaryProductDataSource      *PrimaryProductDataSource `json:"primaryProductDataSource,omitempty"`
	SupplementalProductDataSource json.RawMessage           `json:"supplementalProductDataSource,omitempty"`
	FileInput                     json.RawMessage           `json:"fileInput,omitempty"`
	Raw                           json.RawMessage           `json:"-"`
}

func (value *DataSource) UnmarshalJSON(data []byte) error {
	type alias DataSource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = DataSource(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Availability string

const (
	AvailabilityUnspecified Availability = "AVAILABILITY_UNSPECIFIED"
	AvailabilityInStock     Availability = "IN_STOCK"
	AvailabilityOutOfStock  Availability = "OUT_OF_STOCK"
	AvailabilityPreorder    Availability = "PREORDER"
	AvailabilityLimited     Availability = "LIMITED_AVAILABILITY"
	AvailabilityBackorder   Availability = "BACKORDER"
)

type Condition string

const (
	ConditionUnspecified Condition = "CONDITION_UNSPECIFIED"
	ConditionNew         Condition = "NEW"
	ConditionRefurbished Condition = "REFURBISHED"
	ConditionUsed        Condition = "USED"
)

type Pause string

const (
	PauseUnspecified Pause = "PAUSE_UNSPECIFIED"
	PauseAds         Pause = "ADS"
	PauseAll         Pause = "ALL"
)

type ProductDestination string

const (
	DestinationUnspecified         ProductDestination = "DESTINATION_ENUM_UNSPECIFIED"
	DestinationShoppingAds         ProductDestination = "SHOPPING_ADS"
	DestinationDisplayAds          ProductDestination = "DISPLAY_ADS"
	DestinationLocalInventoryAds   ProductDestination = "LOCAL_INVENTORY_ADS"
	DestinationFreeListings        ProductDestination = "FREE_LISTINGS"
	DestinationFreeLocalListings   ProductDestination = "FREE_LOCAL_LISTINGS"
	DestinationYouTubeShopping     ProductDestination = "YOUTUBE_SHOPPING"
	DestinationYouTubeCheckout     ProductDestination = "YOUTUBE_SHOPPING_CHECKOUT"
	DestinationYouTubeAffiliate    ProductDestination = "YOUTUBE_AFFILIATE"
	DestinationFreeVehicleListings ProductDestination = "FREE_VEHICLE_LISTINGS"
	DestinationVehicleAds          ProductDestination = "VEHICLE_ADS"
	DestinationCloudRetail         ProductDestination = "CLOUD_RETAIL"
	DestinationLocalCloudRetail    ProductDestination = "LOCAL_CLOUD_RETAIL"
)

// Price uses a decimal string for micros to avoid float conversion.
type Price struct {
	AmountMicros string `json:"amountMicros"`
	CurrencyCode string `json:"currencyCode"`
}

type CustomAttribute struct {
	Name        string            `json:"name"`
	Value       string            `json:"value,omitempty"`
	GroupValues []CustomAttribute `json:"groupValues,omitempty"`
}

// ProductAttributes covers commonly used Shopping Ads fields. Extra retains
// every unmodeled v1 field as exact JSON without allowing known-field
// overrides.
type ProductAttributes struct {
	Title                 string                     `json:"title,omitempty"`
	Description           string                     `json:"description,omitempty"`
	Link                  string                     `json:"link,omitempty"`
	CanonicalLink         string                     `json:"canonicalLink,omitempty"`
	ImageLink             string                     `json:"imageLink,omitempty"`
	AdditionalImageLinks  []string                   `json:"additionalImageLinks,omitempty"`
	VideoLinks            []string                   `json:"videoLinks,omitempty"`
	Availability          Availability               `json:"availability,omitempty"`
	AvailabilityDate      string                     `json:"availabilityDate,omitempty"`
	Condition             Condition                  `json:"condition,omitempty"`
	Price                 *Price                     `json:"price,omitempty"`
	SalePrice             *Price                     `json:"salePrice,omitempty"`
	Brand                 string                     `json:"brand,omitempty"`
	GTINs                 []string                   `json:"gtins,omitempty"`
	MPN                   string                     `json:"mpn,omitempty"`
	GoogleProductCategory string                     `json:"googleProductCategory,omitempty"`
	ProductTypes          []string                   `json:"productTypes,omitempty"`
	ItemGroupID           string                     `json:"itemGroupId,omitempty"`
	Color                 string                     `json:"color,omitempty"`
	Size                  string                     `json:"size,omitempty"`
	Adult                 *bool                      `json:"adult,omitempty"`
	CustomLabel0          string                     `json:"customLabel0,omitempty"`
	CustomLabel1          string                     `json:"customLabel1,omitempty"`
	CustomLabel2          string                     `json:"customLabel2,omitempty"`
	CustomLabel3          string                     `json:"customLabel3,omitempty"`
	CustomLabel4          string                     `json:"customLabel4,omitempty"`
	IncludedDestinations  []ProductDestination       `json:"includedDestinations,omitempty"`
	ExcludedDestinations  []ProductDestination       `json:"excludedDestinations,omitempty"`
	Pause                 Pause                      `json:"pause,omitempty"`
	ExpirationDate        string                     `json:"expirationDate,omitempty"`
	Extra                 map[string]json.RawMessage `json:"-"`
}

type ProductInput struct {
	Name                 string             `json:"name,omitempty"`
	Product              string             `json:"product,omitempty"`
	Base64EncodedName    string             `json:"base64EncodedName,omitempty"`
	Base64EncodedProduct string             `json:"base64EncodedProduct,omitempty"`
	OfferID              string             `json:"offerId,omitempty"`
	ContentLanguage      string             `json:"contentLanguage,omitempty"`
	FeedLabel            string             `json:"feedLabel,omitempty"`
	LegacyLocal          bool               `json:"legacyLocal,omitempty"`
	VersionNumber        string             `json:"versionNumber,omitempty"`
	ProductAttributes    *ProductAttributes `json:"productAttributes,omitempty"`
	CustomAttributes     []CustomAttribute  `json:"customAttributes,omitempty"`
	Raw                  json.RawMessage    `json:"-"`
}

func (value *ProductInput) UnmarshalJSON(data []byte) error {
	type alias ProductInput
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = ProductInput(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ItemIssueSeverity string

const (
	ItemNotImpacted ItemIssueSeverity = "NOT_IMPACTED"
	ItemDemoted     ItemIssueSeverity = "DEMOTED"
	ItemDisapproved ItemIssueSeverity = "DISAPPROVED"
)

type ItemLevelIssue struct {
	Code                string            `json:"code"`
	Severity            ItemIssueSeverity `json:"severity"`
	Resolution          string            `json:"resolution"`
	Attribute           string            `json:"attribute"`
	ReportingContext    ReportingContext  `json:"reportingContext"`
	Description         string            `json:"description"`
	Detail              string            `json:"detail"`
	Documentation       string            `json:"documentation"`
	ApplicableCountries []string          `json:"applicableCountries"`
}

type DestinationStatus struct {
	ReportingContext     ReportingContext `json:"reportingContext"`
	ApprovedCountries    []string         `json:"approvedCountries"`
	PendingCountries     []string         `json:"pendingCountries"`
	DisapprovedCountries []string         `json:"disapprovedCountries"`
}

type ProductStatus struct {
	DestinationStatuses  []DestinationStatus `json:"destinationStatuses"`
	ItemLevelIssues      []ItemLevelIssue    `json:"itemLevelIssues"`
	CreationDate         string              `json:"creationDate"`
	LastUpdateDate       string              `json:"lastUpdateDate"`
	GoogleExpirationDate string              `json:"googleExpirationDate"`
}

type Product struct {
	Name              string             `json:"name"`
	Base64EncodedName string             `json:"base64EncodedName"`
	OfferID           string             `json:"offerId"`
	ContentLanguage   string             `json:"contentLanguage"`
	FeedLabel         string             `json:"feedLabel"`
	DataSource        string             `json:"dataSource"`
	LegacyLocal       bool               `json:"legacyLocal"`
	Archived          bool               `json:"archived"`
	VersionNumber     string             `json:"versionNumber"`
	ProductAttributes *ProductAttributes `json:"productAttributes"`
	CustomAttributes  []CustomAttribute  `json:"customAttributes"`
	ProductStatus     ProductStatus      `json:"productStatus"`
	Raw               json.RawMessage    `json:"-"`
}

func (value *Product) UnmarshalJSON(data []byte) error {
	type alias Product
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Product(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type InsertProductInputRequest struct {
	DataSource string
	Input      ProductInput
}

type PatchProductInputRequest struct {
	DataSource string
	Name       string
	Input      ProductInput
	UpdateMask []string
}

type DeleteProductInputRequest struct {
	DataSource string
	Name       string
}

type ReportRequest struct {
	Query     string
	PageSize  int
	PageToken string
}

// ReportRow preserves the single populated Merchant report view as exact JSON.
type ReportRow map[string]json.RawMessage

func (row ReportRow) Field(name string) (json.RawMessage, bool) {
	value, found := row[name]
	if !found {
		return nil, false
	}
	return append(json.RawMessage(nil), value...), true
}

func (row ReportRow) DecodeField(name string, output any) error {
	value, found := row[name]
	if !found {
		return fmt.Errorf("merchantapi: report field %q is absent", name)
	}
	if output == nil {
		return fmt.Errorf("merchantapi: report field destination is required")
	}
	return json.Unmarshal(value, output)
}

type ReportPage struct {
	Rows          []ReportRow
	NextPageToken string
}

type MethodDetails struct {
	Method  string `json:"method"`
	SubAPI  string `json:"subapi"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

type QuotaGroup struct {
	Name             string          `json:"name"`
	QuotaUsage       string          `json:"quotaUsage"`
	QuotaLimit       string          `json:"quotaLimit"`
	QuotaMinuteLimit string          `json:"quotaMinuteLimit"`
	MethodDetails    []MethodDetails `json:"methodDetails"`
	Raw              json.RawMessage `json:"-"`
}

func (value *QuotaGroup) UnmarshalJSON(data []byte) error {
	type alias QuotaGroup
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = QuotaGroup(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ProductLimit struct {
	Scope string `json:"scope"`
	Limit string `json:"limit"`
}

type AccountLimit struct {
	Name     string          `json:"name"`
	Products *ProductLimit   `json:"products,omitempty"`
	Raw      json.RawMessage `json:"-"`
}

func (value *AccountLimit) UnmarshalJSON(data []byte) error {
	type alias AccountLimit
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = AccountLimit(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

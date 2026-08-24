package branch

import (
	"context"
	"encoding/json"
	"fmt"

	"social-hub/pkg/socialhub"
)

// Decimal preserves an exact finite JSON number without float64 rounding.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("branch: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

type StandardEventName string

const (
	EventAddToCart            StandardEventName = "ADD_TO_CART"
	EventAddToWishlist        StandardEventName = "ADD_TO_WISHLIST"
	EventClickAd              StandardEventName = "CLICK_AD"
	EventViewCart             StandardEventName = "VIEW_CART"
	EventInitiatePurchase     StandardEventName = "INITIATE_PURCHASE"
	EventAddPaymentInfo       StandardEventName = "ADD_PAYMENT_INFO"
	EventPurchase             StandardEventName = "PURCHASE"
	EventSpendCredits         StandardEventName = "SPEND_CREDITS"
	EventViewAd               StandardEventName = "VIEW_AD"
	EventSearch               StandardEventName = "SEARCH"
	EventViewItem             StandardEventName = "VIEW_ITEM"
	EventViewItems            StandardEventName = "VIEW_ITEMS"
	EventRate                 StandardEventName = "RATE"
	EventShare                StandardEventName = "SHARE"
	EventInitiateStream       StandardEventName = "INITIATE_STREAM"
	EventCompleteStream       StandardEventName = "COMPLETE_STREAM"
	EventCompleteRegistration StandardEventName = "COMPLETE_REGISTRATION"
	EventCompleteTutorial     StandardEventName = "COMPLETE_TUTORIAL"
	EventAchieveLevel         StandardEventName = "ACHIEVE_LEVEL"
	EventUnlockAchievement    StandardEventName = "UNLOCK_ACHIEVEMENT"
	EventInvite               StandardEventName = "INVITE"
	EventLogin                StandardEventName = "LOGIN"
	EventStartTrial           StandardEventName = "START_TRIAL"
	EventSubscribe            StandardEventName = "SUBSCRIBE"
)

type OperatingSystem string

const (
	OSAndroid OperatingSystem = "Android"
	OSIOS     OperatingSystem = "iOS"
	OSMac     OperatingSystem = "Mac_OS"
	OSLinux   OperatingSystem = "Linux"
	OSWindows OperatingSystem = "Windows"
)

type Environment string

const (
	EnvironmentFullApp    Environment = "FULL_APP"
	EnvironmentInstantApp Environment = "INSTANT_APP"
)

// Properties is a flat, scalar JSON object used by Branch custom fields.
type Properties struct {
	Strings  map[string]string
	Numbers  map[string]Decimal
	Booleans map[string]bool
}

type UserData struct {
	OS                    OperatingSystem
	OSVersion             string
	AdvertisingIDs        map[string]string
	Environment           Environment
	AAID                  string
	AndroidID             string
	IDFA                  string
	IDFV                  string
	AnonID                string
	LimitAdTracking       *bool
	UserAgent             string
	BrowserFingerprintID  string
	HTTPOrigin            string
	HTTPReferrer          string
	DeveloperIdentity     string
	GoogleAnalyticsID     string
	RandomizedDeviceToken string
	Country               string
	Language              string
	IP                    string
	LocalIP               string
	Brand                 string
	AppVersion            string
	Model                 string
	ScreenDPI             *int64
	ScreenHeight          *int64
	ScreenWidth           *int64
	DMAEEA                *bool
	DMAAdPersonalization  *bool
	DMAAdUserData         *bool
}

type EventData struct {
	TransactionID string
	Revenue       Decimal
	Currency      string
	Shipping      Decimal
	Tax           Decimal
	Coupon        string
	Affiliation   string
	Description   string
	SearchQuery   string
}

type ContentSchema string

const (
	SchemaCommerceAuction      ContentSchema = "COMMERCE_AUCTION"
	SchemaCommerceBusiness     ContentSchema = "COMMERCE_BUSINESS"
	SchemaCommerceOther        ContentSchema = "COMMERCE_OTHER"
	SchemaCommerceProduct      ContentSchema = "COMMERCE_PRODUCT"
	SchemaCommerceRestaurant   ContentSchema = "COMMERCE_RESTAURANT"
	SchemaCommerceService      ContentSchema = "COMMERCE_SERVICE"
	SchemaCommerceTravelFlight ContentSchema = "COMMERCE_TRAVEL_FLIGHT"
	SchemaCommerceTravelHotel  ContentSchema = "COMMERCE_TRAVEL_HOTEL"
	SchemaCommerceTravelOther  ContentSchema = "COMMERCE_TRAVEL_OTHER"
	SchemaGameState            ContentSchema = "GAME_STATE"
	SchemaMediaImage           ContentSchema = "MEDIA_IMAGE"
	SchemaMediaMixed           ContentSchema = "MEDIA_MIXED"
	SchemaMediaMusic           ContentSchema = "MEDIA_MUSIC"
	SchemaMediaOther           ContentSchema = "MEDIA_OTHER"
	SchemaMediaVideo           ContentSchema = "MEDIA_VIDEO"
	SchemaOther                ContentSchema = "OTHER"
	SchemaTextArticle          ContentSchema = "TEXT_ARTICLE"
	SchemaTextBlog             ContentSchema = "TEXT_BLOG"
	SchemaTextOther            ContentSchema = "TEXT_OTHER"
	SchemaTextRecipe           ContentSchema = "TEXT_RECIPE"
	SchemaTextReview           ContentSchema = "TEXT_REVIEW"
	SchemaTextSearchResults    ContentSchema = "TEXT_SEARCH_RESULTS"
	SchemaTextStory            ContentSchema = "TEXT_STORY"
	SchemaTextTechnicalDoc     ContentSchema = "TEXT_TECHNICAL_DOC"
)

type ProductCategory string

const (
	ProductAnimalsPetSupplies   ProductCategory = "ANIMALS_AND_PET_SUPPLIES"
	ProductApparelAccessories   ProductCategory = "APPAREL_AND_ACCESSORIES"
	ProductArtsEntertainment    ProductCategory = "ARTS_AND_ENTERTAINMENT"
	ProductBabyToddler          ProductCategory = "BABY_AND_TODDLER"
	ProductBusinessIndustrial   ProductCategory = "BUSINESS_AND_INDUSTRIAL"
	ProductCamerasOptics        ProductCategory = "CAMERAS_AND_OPTICS"
	ProductElectronics          ProductCategory = "ELECTRONICS"
	ProductFoodBeveragesTobacco ProductCategory = "FOOD_BEVERAGES_AND_TOBACCO"
	ProductFurniture            ProductCategory = "FURNITURE"
	ProductHardware             ProductCategory = "HARDWARE"
	ProductHealthBeauty         ProductCategory = "HEALTH_AND_BEAUTY"
	ProductHomeGarden           ProductCategory = "HOME_AND_GARDEN"
	ProductLuggageBags          ProductCategory = "LUGGAGE_AND_BAGS"
	ProductMature               ProductCategory = "MATURE"
	ProductMedia                ProductCategory = "MEDIA"
	ProductOfficeSupplies       ProductCategory = "OFFICE_SUPPLIES"
	ProductReligiousCeremonial  ProductCategory = "RELIGIOUS_AND_CEREMONIAL"
	ProductSoftware             ProductCategory = "SOFTWARE"
	ProductSportingGoods        ProductCategory = "SPORTING_GOODS"
	ProductToysGames            ProductCategory = "TOYS_AND_GAMES"
	ProductVehiclesParts        ProductCategory = "VEHICLES_AND_PARTS"
)

type ContentCondition string

const (
	ConditionOther       ContentCondition = "OTHER"
	ConditionNew         ContentCondition = "NEW"
	ConditionExcellent   ContentCondition = "EXCELLENT"
	ConditionGood        ContentCondition = "GOOD"
	ConditionFair        ContentCondition = "FAIR"
	ConditionPoor        ContentCondition = "POOR"
	ConditionUsed        ContentCondition = "USED"
	ConditionRefurbished ContentCondition = "REFURBISHED"
)

type ContentItem struct {
	Schema              ContentSchema
	OGTitle             string
	OGDescription       string
	OGImageURL          string
	CanonicalIdentifier string
	PubliclyIndexable   *bool
	LocallyIndexable    *bool
	Price               Decimal
	Quantity            Decimal
	SKU                 string
	ProductName         string
	ProductBrand        string
	ProductCategory     ProductCategory
	ProductVariant      string
	RatingAverage       Decimal
	RatingCount         Decimal
	RatingMax           Decimal
	CreationTimestamp   *int64
	ExpirationTimestamp *int64
	Keywords            []string
	AddressStreet       string
	AddressCity         string
	AddressRegion       string
	AddressCountry      string
	AddressPostalCode   string
	Latitude            Decimal
	Longitude           Decimal
	ImageCaptions       []string
	Condition           ContentCondition
	CustomFields        Properties
}

type StandardEventRequest struct {
	Name               StandardEventName
	CustomerEventAlias string
	UserData           UserData
	CustomData         Properties
	EventData          *EventData
	ContentItems       []ContentItem
	IPOverride         string
}

type CustomEventRequest struct {
	Name       string
	UserData   UserData
	CustomData Properties
	Metadata   Properties
	EventData  *EventData
	IPOverride string
}

type SubmitResult struct {
	StatusCode            int
	AscendingOnly         *bool
	CoarseKey             *string
	Locked                *bool
	UpdateConversionValue *int64
}

type EventWorkflow interface {
	TrackStandardEvent(context.Context, StandardEventRequest, ...socialhub.CallOption) (SubmitResult, error)
	TrackCustomEvent(context.Context, CustomEventRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)

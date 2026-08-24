package googledatamanager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumEventsPerRequest       = 2000
	MaximumDestinationsPerRequest = 10
	MaximumUserIdentifiers        = 10
)

// Decimal preserves an exact JSON decimal instead of converting monetary
// values through binary floating point.
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("googledatamanager: invalid decimal %q", value)
	}
	return []byte(value), nil
}

type Encoding string

const (
	EncodingHex    Encoding = "HEX"
	EncodingBase64 Encoding = "BASE64"
)

type AccountType string

const (
	AccountTypeGoogleAds               AccountType = "GOOGLE_ADS"
	AccountTypeDisplayVideoPartner     AccountType = "DISPLAY_VIDEO_PARTNER"
	AccountTypeDisplayVideoAdvertiser  AccountType = "DISPLAY_VIDEO_ADVERTISER"
	AccountTypeDataPartner             AccountType = "DATA_PARTNER"
	AccountTypeGoogleAnalyticsProperty AccountType = "GOOGLE_ANALYTICS_PROPERTY"
	AccountTypeGoogleAdManagerAudience AccountType = "GOOGLE_AD_MANAGER_AUDIENCE_LINK"
	AccountTypeFloodlightConfig        AccountType = "FLOODLIGHT_CONFIG"
)

type ProductAccount struct {
	AccountID   string      `json:"accountId"`
	AccountType AccountType `json:"accountType"`
}

type Destination struct {
	Reference            string          `json:"reference,omitempty"`
	LoginAccount         *ProductAccount `json:"loginAccount,omitempty"`
	LinkedAccount        *ProductAccount `json:"linkedAccount,omitempty"`
	OperatingAccount     ProductAccount  `json:"operatingAccount"`
	ProductDestinationID string          `json:"productDestinationId"`
}

type ConsentStatus string

const (
	ConsentGranted ConsentStatus = "CONSENT_GRANTED"
	ConsentDenied  ConsentStatus = "CONSENT_DENIED"
)

type Consent struct {
	AdUserData        ConsentStatus `json:"adUserData,omitempty"`
	AdPersonalization ConsentStatus `json:"adPersonalization,omitempty"`
}

type KeyType string

const KeyTypeXChaCha20Poly1305 KeyType = "XCHACHA20_POLY1305"

// EncryptionInfo deliberately omits coordinator keys because events:ingest
// does not support them. Exactly one wrapped-key provider must be configured.
type EncryptionInfo struct {
	GCPWrappedKeyInfo *GCPWrappedKeyInfo `json:"gcpWrappedKeyInfo,omitempty"`
	AWSWrappedKeyInfo *AWSWrappedKeyInfo `json:"awsWrappedKeyInfo,omitempty"`
}

type GCPWrappedKeyInfo struct {
	KeyType      KeyType `json:"keyType"`
	WIPProvider  string  `json:"wipProvider"`
	KEKURI       string  `json:"kekUri"`
	EncryptedDEK string  `json:"encryptedDek"`
}

type AWSWrappedKeyInfo struct {
	KeyType      KeyType `json:"keyType"`
	RoleARN      string  `json:"roleArn"`
	KEKURI       string  `json:"kekUri"`
	EncryptedDEK string  `json:"encryptedDek"`
}

type IngestEventsRequest struct {
	Destinations   []Destination   `json:"destinations"`
	Events         []Event         `json:"events"`
	Consent        *Consent        `json:"consent,omitempty"`
	ValidateOnly   bool            `json:"validateOnly,omitempty"`
	Encoding       Encoding        `json:"encoding,omitempty"`
	EncryptionInfo *EncryptionInfo `json:"encryptionInfo,omitempty"`
}

type WarningReason string

const (
	WarningCustomVariableNotEnabled      WarningReason = "WARNING_REASON_CUSTOM_VARIABLE_NOT_ENABLED"
	WarningCustomVariableNotPredefined   WarningReason = "WARNING_REASON_CUSTOM_VARIABLE_NOT_PREDEFINED"
	WarningCartDataNotSupportedWithBraid WarningReason = "WARNING_REASON_CART_DATA_NOT_SUPPORTED_WITH_GBRAID_OR_WBRAID"
	WarningCartItemProductIDMissing      WarningReason = "WARNING_REASON_CART_DATA_ITEM_MERCHANT_PRODUCT_ID_MISSING"
	WarningCartItemUnitPriceMissing      WarningReason = "WARNING_REASON_CART_DATA_ITEM_UNIT_PRICE_MISSING"
	WarningGeneric                       WarningReason = "WARNING_REASON_GENERIC"
	WarningInvalidClientID               WarningReason = "WARNING_REASON_INVALID_CLIENT_ID"
	WarningInvalidSubdivisionCode        WarningReason = "WARNING_REASON_INVALID_SUBDIVISION_CODE"
	WarningInvalidRegionCode             WarningReason = "WARNING_REASON_INVALID_REGION_CODE"
	WarningInvalidSubcontinentCode       WarningReason = "WARNING_REASON_INVALID_SUBCONTINENT_CODE"
	WarningInvalidContinentCode          WarningReason = "WARNING_REASON_INVALID_CONTINENT_CODE"
	WarningInvalidDeviceCategory         WarningReason = "WARNING_REASON_INVALID_DEVICE_CATEGORY"
	WarningInvalidDeviceScreenResolution WarningReason = "WARNING_REASON_INVALID_DEVICE_SCREEN_RESOLUTION"
	WarningInvalidMerchantID             WarningReason = "WARNING_REASON_INVALID_MERCHANT_ID"
)

type FieldWarning struct {
	Reason      WarningReason `json:"reason,omitempty"`
	Description string        `json:"description,omitempty"`
	Field       string        `json:"field,omitempty"`
}

type IngestEventsResponse struct {
	RequestID     string         `json:"requestId"`
	FieldWarnings []FieldWarning `json:"fieldWarnings,omitempty"`
}

type EventSource string

const (
	EventSourceWeb     EventSource = "WEB"
	EventSourceApp     EventSource = "APP"
	EventSourceInStore EventSource = "IN_STORE"
	EventSourcePhone   EventSource = "PHONE"
	EventSourceMessage EventSource = "MESSAGE"
	EventSourceOther   EventSource = "OTHER"
)

type Event struct {
	DestinationReferences     []string            `json:"destinationReferences,omitempty"`
	TransactionID             string              `json:"transactionId,omitempty"`
	EventTimestamp            time.Time           `json:"eventTimestamp"`
	LastUpdatedTimestamp      *time.Time          `json:"lastUpdatedTimestamp,omitempty"`
	UserData                  *UserData           `json:"userData,omitempty"`
	ThirdPartyUserData        *UserData           `json:"thirdPartyUserData,omitempty"`
	Consent                   *Consent            `json:"consent,omitempty"`
	AdIdentifiers             *AdIdentifiers      `json:"adIdentifiers,omitempty"`
	Currency                  string              `json:"currency,omitempty"`
	ConversionValue           Decimal             `json:"conversionValue,omitempty"`
	ConversionCount           Decimal             `json:"conversionCount,omitempty"`
	EventSource               EventSource         `json:"eventSource,omitempty"`
	EventDeviceInfo           *DeviceInfo         `json:"eventDeviceInfo,omitempty"`
	EventLocation             *EventLocation      `json:"eventLocation,omitempty"`
	CartData                  *CartData           `json:"cartData,omitempty"`
	CustomVariables           []CustomVariable    `json:"customVariables,omitempty"`
	ExperimentalFields        []ExperimentalField `json:"experimentalFields,omitempty"`
	UserProperties            *UserProperties     `json:"userProperties,omitempty"`
	ClientID                  string              `json:"clientId,omitempty"`
	AppInstanceID             string              `json:"appInstanceId,omitempty"`
	UserID                    string              `json:"userId,omitempty"`
	EventName                 string              `json:"eventName,omitempty"`
	AdditionalEventParameters []EventParameter    `json:"additionalEventParameters,omitempty"`
}

type UserData struct {
	UserIdentifiers []UserIdentifier `json:"userIdentifiers"`
}

// UserIdentifier is a oneof: set exactly one field. Plain values are
// normalized, SHA-256 hashed, and encoded by the SDK. Already encoded SHA-256
// values are preserved. With EncryptionInfo, fields that require hashing are
// treated as caller-encrypted ciphertext.
type UserIdentifier struct {
	EmailAddress string       `json:"emailAddress,omitempty"`
	PhoneNumber  string       `json:"phoneNumber,omitempty"`
	Address      *AddressInfo `json:"address,omitempty"`
}

type AddressInfo struct {
	GivenName          string `json:"givenName"`
	FamilyName         string `json:"familyName"`
	RegionCode         string `json:"regionCode"`
	PostalCode         string `json:"postalCode"`
	AddressLine        string `json:"addressLine,omitempty"`
	City               string `json:"city,omitempty"`
	AdministrativeArea string `json:"administrativeArea,omitempty"`
}

type AdIdentifiers struct {
	SessionAttributes     string            `json:"sessionAttributes,omitempty"`
	GCLID                 string            `json:"gclid,omitempty"`
	GBRAID                string            `json:"gbraid,omitempty"`
	WBRAID                string            `json:"wbraid,omitempty"`
	LandingPageDeviceInfo *DeviceInfo       `json:"landingPageDeviceInfo,omitempty"`
	MobileDeviceID        string            `json:"mobileDeviceId,omitempty"`
	DCLID                 string            `json:"dclid,omitempty"`
	ImpressionID          string            `json:"impressionId,omitempty"`
	MatchID               string            `json:"matchId,omitempty"`
	EncryptedUserIDs      []EncryptedUserID `json:"encryptedUserIds,omitempty"`
}

type EncryptionEntityType string

const (
	EncryptionEntityCampaignManagerAccount    EncryptionEntityType = "CAMPAIGN_MANAGER_ACCOUNT"
	EncryptionEntityCampaignManagerAdvertiser EncryptionEntityType = "CAMPAIGN_MANAGER_ADVERTISER"
	EncryptionEntityDisplayVideoPartner       EncryptionEntityType = "DISPLAY_VIDEO_PARTNER"
	EncryptionEntityDisplayVideoAdvertiser    EncryptionEntityType = "DISPLAY_VIDEO_ADVERTISER"
	EncryptionEntityGoogleAdsCustomer         EncryptionEntityType = "GOOGLE_ADS_CUSTOMER"
	EncryptionEntityGoogleAdManagerNetwork    EncryptionEntityType = "GOOGLE_AD_MANAGER_NETWORK_CODE"
)

type EncryptionSource string

const (
	EncryptionSourceAdServing    EncryptionSource = "AD_SERVING"
	EncryptionSourceDataTransfer EncryptionSource = "DATA_TRANSFER"
)

type EncryptedUserID struct {
	EncryptedID string               `json:"encryptedId"`
	EntityType  EncryptionEntityType `json:"entityType"`
	EntityID    string               `json:"entityId"`
	Source      EncryptionSource     `json:"source"`
}

type DeviceInfo struct {
	UserAgent              string `json:"userAgent,omitempty"`
	IPAddress              string `json:"ipAddress,omitempty"`
	Category               string `json:"category,omitempty"`
	LanguageCode           string `json:"languageCode,omitempty"`
	ScreenHeight           int32  `json:"screenHeight,omitempty"`
	ScreenWidth            int32  `json:"screenWidth,omitempty"`
	OperatingSystem        string `json:"operatingSystem,omitempty"`
	OperatingSystemVersion string `json:"operatingSystemVersion,omitempty"`
	Model                  string `json:"model,omitempty"`
	Brand                  string `json:"brand,omitempty"`
	Browser                string `json:"browser,omitempty"`
	BrowserVersion         string `json:"browserVersion,omitempty"`
}

type CartData struct {
	MerchantID               string   `json:"merchantId,omitempty"`
	MerchantFeedLabel        string   `json:"merchantFeedLabel,omitempty"`
	MerchantFeedLanguageCode string   `json:"merchantFeedLanguageCode,omitempty"`
	TransactionDiscount      Decimal  `json:"transactionDiscount,omitempty"`
	Items                    []Item   `json:"items,omitempty"`
	CouponCodes              []string `json:"couponCodes,omitempty"`
}

type Item struct {
	MerchantProductID        string               `json:"merchantProductId,omitempty"`
	Quantity                 string               `json:"quantity,omitempty"`
	UnitPrice                Decimal              `json:"unitPrice,omitempty"`
	ItemID                   string               `json:"itemId,omitempty"`
	AdditionalItemParameters []ItemParameter      `json:"additionalItemParameters,omitempty"`
	MerchantID               string               `json:"merchantId,omitempty"`
	MerchantFeedLabel        string               `json:"merchantFeedLabel,omitempty"`
	MerchantFeedLanguageCode string               `json:"merchantFeedLanguageCode,omitempty"`
	CustomVariables          []ItemCustomVariable `json:"customVariables,omitempty"`
	ConversionValue          Decimal              `json:"conversionValue,omitempty"`
}

type ItemParameter struct {
	ParameterName string `json:"parameterName"`
	Value         string `json:"value"`
}

type ItemCustomVariable struct {
	Variable              string   `json:"variable,omitempty"`
	Value                 string   `json:"value,omitempty"`
	DestinationReferences []string `json:"destinationReferences,omitempty"`
}

type CustomVariable struct {
	Variable              string   `json:"variable,omitempty"`
	Value                 string   `json:"value,omitempty"`
	DestinationReferences []string `json:"destinationReferences,omitempty"`
}

type ExperimentalField struct {
	Field string `json:"field,omitempty"`
	Value string `json:"value,omitempty"`
}

type CustomerType string

const (
	CustomerTypeNew       CustomerType = "NEW"
	CustomerTypeReturning CustomerType = "RETURNING"
	CustomerTypeReengaged CustomerType = "REENGAGED"
)

type CustomerValueBucket string

const (
	CustomerValueLow    CustomerValueBucket = "LOW"
	CustomerValueMedium CustomerValueBucket = "MEDIUM"
	CustomerValueHigh   CustomerValueBucket = "HIGH"
)

type UserProperties struct {
	CustomerType             CustomerType        `json:"customerType,omitempty"`
	CustomerValueBucket      CustomerValueBucket `json:"customerValueBucket,omitempty"`
	AdditionalUserProperties []UserProperty      `json:"additionalUserProperties,omitempty"`
}

type UserProperty struct {
	PropertyName string `json:"propertyName"`
	Value        string `json:"value"`
}

type EventParameter struct {
	ParameterName string `json:"parameterName"`
	Value         string `json:"value"`
}

type EventLocation struct {
	StoreID          string `json:"storeId,omitempty"`
	City             string `json:"city,omitempty"`
	SubdivisionCode  string `json:"subdivisionCode,omitempty"`
	RegionCode       string `json:"regionCode,omitempty"`
	SubcontinentCode string `json:"subcontinentCode,omitempty"`
	ContinentCode    string `json:"continentCode,omitempty"`
}

type EventIngestor interface {
	IngestEvents(context.Context, IngestEventsRequest, ...socialhub.CallOption) (*IngestEventsResponse, error)
}

var (
	_ json.Marshaler = Decimal("")
	_ EventIngestor  = (*Client)(nil)
)

package admanager

import "encoding/json"

type ListRequest struct {
	PageSize  int32
	PageToken string
	Filter    string
	OrderBy   string
	Skip      int32
}

type Page[T any] struct {
	Items         []T
	NextPageToken string
	TotalSize     int32
}

type Network struct {
	Name                   string          `json:"name"`
	NetworkCode            string          `json:"networkCode,omitempty"`
	NetworkID              string          `json:"networkId,omitempty"`
	DisplayName            string          `json:"displayName,omitempty"`
	PropertyCode           string          `json:"propertyCode,omitempty"`
	TimeZone               string          `json:"timeZone,omitempty"`
	CurrencyCode           string          `json:"currencyCode,omitempty"`
	SecondaryCurrencyCodes []string        `json:"secondaryCurrencyCodes,omitempty"`
	EffectiveRootAdUnit    string          `json:"effectiveRootAdUnit,omitempty"`
	TestNetwork            bool            `json:"testNetwork,omitempty"`
	Raw                    json.RawMessage `json:"-"`
}

type CompanyType string

const (
	CompanyAdvertiser      CompanyType = "ADVERTISER"
	CompanyHouseAdvertiser CompanyType = "HOUSE_ADVERTISER"
	CompanyAgency          CompanyType = "AGENCY"
	CompanyHouseAgency     CompanyType = "HOUSE_AGENCY"
	CompanyAdNetwork       CompanyType = "AD_NETWORK"
)

type Company struct {
	Name           string          `json:"name"`
	CompanyID      string          `json:"companyId,omitempty"`
	DisplayName    string          `json:"displayName,omitempty"`
	Type           CompanyType     `json:"type,omitempty"`
	ExternalID     string          `json:"externalId,omitempty"`
	Email          string          `json:"email,omitempty"`
	Phone          string          `json:"phone,omitempty"`
	Address        string          `json:"address,omitempty"`
	CreditStatus   string          `json:"creditStatus,omitempty"`
	PrimaryContact string          `json:"primaryContact,omitempty"`
	UpdateTime     string          `json:"updateTime,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

type Size struct {
	Width    int32  `json:"width,omitempty"`
	Height   int32  `json:"height,omitempty"`
	SizeType string `json:"sizeType,omitempty"`
}

type AdUnitSize struct {
	EnvironmentType string `json:"environmentType,omitempty"`
	Size            Size   `json:"size"`
	Companions      []Size `json:"companions,omitempty"`
}

type AdUnitParent struct {
	ParentAdUnit string `json:"parentAdUnit,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	AdUnitCode   string `json:"adUnitCode,omitempty"`
}

type AdUnitStatus string

const (
	AdUnitActive   AdUnitStatus = "ACTIVE"
	AdUnitInactive AdUnitStatus = "INACTIVE"
	AdUnitArchived AdUnitStatus = "ARCHIVED"
)

type AdUnit struct {
	Name               string          `json:"name"`
	AdUnitID           string          `json:"adUnitId,omitempty"`
	DisplayName        string          `json:"displayName,omitempty"`
	AdUnitCode         string          `json:"adUnitCode,omitempty"`
	ParentAdUnit       string          `json:"parentAdUnit,omitempty"`
	ParentPath         []AdUnitParent  `json:"parentPath,omitempty"`
	Status             AdUnitStatus    `json:"status,omitempty"`
	Description        string          `json:"description,omitempty"`
	AdUnitSizes        []AdUnitSize    `json:"adUnitSizes,omitempty"`
	HasChildren        bool            `json:"hasChildren,omitempty"`
	ExplicitlyTargeted bool            `json:"explicitlyTargeted,omitempty"`
	UpdateTime         string          `json:"updateTime,omitempty"`
	Raw                json.RawMessage `json:"-"`
}

type OrderStatus string

const (
	OrderDraft           OrderStatus = "DRAFT"
	OrderPendingApproval OrderStatus = "PENDING_APPROVAL"
	OrderApproved        OrderStatus = "APPROVED"
	OrderDisapproved     OrderStatus = "DISAPPROVED"
	OrderPaused          OrderStatus = "PAUSED"
	OrderCanceled        OrderStatus = "CANCELED"
	OrderDeleted         OrderStatus = "DELETED"
)

type Order struct {
	Name                              string          `json:"name"`
	OrderID                           string          `json:"orderId,omitempty"`
	DisplayName                       string          `json:"displayName,omitempty"`
	Programmatic                      bool            `json:"programmatic,omitempty"`
	Trafficker                        string          `json:"trafficker,omitempty"`
	Advertiser                        string          `json:"advertiser,omitempty"`
	Agency                            string          `json:"agency,omitempty"`
	Creator                           string          `json:"creator,omitempty"`
	CurrencyCode                      string          `json:"currencyCode,omitempty"`
	StartTime                         string          `json:"startTime,omitempty"`
	EndTime                           string          `json:"endTime,omitempty"`
	UnlimitedEndTime                  bool            `json:"unlimitedEndTime,omitempty"`
	ExternalOrderID                   int32           `json:"externalOrderId,omitempty"`
	Archived                          bool            `json:"archived,omitempty"`
	Notes                             string          `json:"notes,omitempty"`
	PONumber                          string          `json:"poNumber,omitempty"`
	Status                            OrderStatus     `json:"status,omitempty"`
	Salesperson                       string          `json:"salesperson,omitempty"`
	ImpressionsDelivered              string          `json:"impressionsDelivered,omitempty"`
	TotalClicksDelivered              string          `json:"totalClicksDelivered,omitempty"`
	TotalViewableImpressionsDelivered string          `json:"totalViewableImpressionsDelivered,omitempty"`
	UpdateTime                        string          `json:"updateTime,omitempty"`
	Raw                               json.RawMessage `json:"-"`
}

type Money struct {
	CurrencyCode string `json:"currencyCode,omitempty"`
	Units        string `json:"units,omitempty"`
	Nanos        int32  `json:"nanos,omitempty"`
}

type LineItemStats struct {
	ImpressionsDelivered         string `json:"impressionsDelivered,omitempty"`
	ClicksDelivered              string `json:"clicksDelivered,omitempty"`
	ViewableImpressionsDelivered string `json:"viewableImpressionsDelivered,omitempty"`
	VideoStartsDelivered         string `json:"videoStartsDelivered,omitempty"`
	VideoCompletionsDelivered    string `json:"videoCompletionsDelivered,omitempty"`
}

type LineItem struct {
	Name               string          `json:"name"`
	Order              string          `json:"order,omitempty"`
	DisplayName        string          `json:"displayName,omitempty"`
	ExternalLineItemID string          `json:"externalLineItemId,omitempty"`
	OrderDisplayName   string          `json:"orderDisplayName,omitempty"`
	StartTime          string          `json:"startTime,omitempty"`
	TargetEndTime      string          `json:"targetEndTime,omitempty"`
	EndTime            string          `json:"endTime,omitempty"`
	EndTimeUnlimited   bool            `json:"endTimeUnlimited,omitempty"`
	LineItemType       string          `json:"lineItemType,omitempty"`
	Priority           int32           `json:"priority,omitempty"`
	Rate               Money           `json:"rate,omitempty"`
	CostType           string          `json:"costType,omitempty"`
	Budget             Money           `json:"budget,omitempty"`
	Status             string          `json:"status,omitempty"`
	ReservationStatus  string          `json:"reservationStatus,omitempty"`
	Archived           bool            `json:"archived,omitempty"`
	MissingCreatives   bool            `json:"missingCreatives,omitempty"`
	Stats              *LineItemStats  `json:"stats,omitempty"`
	CreateTime         string          `json:"createTime,omitempty"`
	UpdateTime         string          `json:"updateTime,omitempty"`
	Raw                json.RawMessage `json:"-"`
}

func (value *Network) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*networkAlias)(value), &value.Raw)
}
func (value *Company) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*companyAlias)(value), &value.Raw)
}
func (value *AdUnit) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*adUnitAlias)(value), &value.Raw)
}
func (value *Order) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*orderAlias)(value), &value.Raw)
}
func (value *LineItem) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*lineItemAlias)(value), &value.Raw)
}

type networkAlias Network
type companyAlias Company
type adUnitAlias AdUnit
type orderAlias Order
type lineItemAlias LineItem

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

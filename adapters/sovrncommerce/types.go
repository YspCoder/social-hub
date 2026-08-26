package sovrncommerce

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxExactValueBytes     = 256
	maxProviderObjectBytes = 8 << 20
)

// ExactValue preserves provider identifiers, counters, and decimal values
// without float64 coercion.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("sovrncommerce: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("sovrncommerce: exact value must be a JSON string, number, or null")
	}
	value.raw = append(value.raw[:0], trimmed...)
	return nil
}

func (value ExactValue) MarshalJSON() ([]byte, error) {
	if len(value.raw) == 0 {
		return []byte("null"), nil
	}
	return append([]byte(nil), value.raw...), nil
}

func (value ExactValue) Bytes() []byte { return append([]byte(nil), value.raw...) }
func (value ExactValue) IsSet() bool   { return len(value.raw) > 0 }
func (value ExactValue) IsNull() bool  { return bytes.Equal(bytes.TrimSpace(value.raw), []byte("null")) }

func (value ExactValue) String() string {
	trimmed := bytes.TrimSpace(value.raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if json.Unmarshal(trimmed, &text) == nil {
			return text
		}
	}
	return string(trimmed)
}

func (value ExactValue) Decode(target any) error {
	if target == nil || len(value.raw) == 0 {
		return fmt.Errorf("sovrncommerce: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID    string
	ETag         string
	LastModified string
	RetryAfter   string
}

type ProgramType string

const (
	ProgramCPA ProgramType = "CPA"
	ProgramCPC ProgramType = "CPC"
)

type SovrnProduct string

const (
	ProductUnknown           SovrnProduct = "UNK"
	ProductJavaScript        SovrnProduct = "ORG"
	ProductInsert            SovrnProduct = "INS"
	ProductLink              SovrnProduct = "WRA"
	ProductClickAPI          SovrnProduct = "RAC"
	ProductLinkAPI           SovrnProduct = "RAL"
	ProductCouponAPI         SovrnProduct = "CUP"
	ProductProductAPI        SovrnProduct = "TPA"
	ProductPriceComparison   SovrnProduct = "PCR"
	ProductInText            SovrnProduct = "TXT"
	ProductShoppingGalleries SovrnProduct = "SHG"
	ProductFeed              SovrnProduct = "PRF"
)

type DeviceType string

const (
	DeviceDesktop DeviceType = "DSK"
	DeviceMobile  DeviceType = "MBL"
	DeviceTablet  DeviceType = "TBL"
	DeviceUnknown DeviceType = "UKN"
)

type MerchantCategory string

const (
	CategoryConsumerElectronics MerchantCategory = "CE"
	CategoryAutomotive          MerchantCategory = "AU"
	CategoryFashion             MerchantCategory = "FS"
	CategoryHealthBeauty        MerchantCategory = "HB"
	CategoryRealEstate          MerchantCategory = "RU"
	CategoryArtEntertainment    MerchantCategory = "AE"
	CategorySportsFitness       MerchantCategory = "SF"
	CategorySelfHelp            MerchantCategory = "SH"
	CategoryTravel              MerchantCategory = "TV"
	CategoryFinancialServices   MerchantCategory = "FI"
	CategoryPets                MerchantCategory = "PT"
	CategoryMobile              MerchantCategory = "CM"
	CategoryBooksMagazines      MerchantCategory = "BK"
	CategoryEducation           MerchantCategory = "ED"
	CategoryOther               MerchantCategory = "OT"
	CategoryDating              MerchantCategory = "DT"
	CategoryMusic               MerchantCategory = "MM"
	CategoryFoodDrink           MerchantCategory = "FD"
	CategoryHomeGarden          MerchantCategory = "HG"
	CategoryAdultGambling       MerchantCategory = "AG"
	CategoryCareerEmployment    MerchantCategory = "CA"
	CategoryCollectibles        MerchantCategory = "CB"
	CategoryOnlineServices      MerchantCategory = "EM"
	CategoryFamilyBaby          MerchantCategory = "FB"
	CategoryFirearmsHunting     MerchantCategory = "FH"
	CategoryGaming              MerchantCategory = "GM"
	CategoryJewelryWatches      MerchantCategory = "JW"
	CategoryLifestyle           MerchantCategory = "LF"
	CategoryMotorcycles         MerchantCategory = "MP"
	CategoryShoppingCoupons     MerchantCategory = "SP"
	CategoryToysHobbies         MerchantCategory = "HO"
	CategoryCamerasPhoto        MerchantCategory = "CP"
	CategoryUndefined           MerchantCategory = "UN"
)

type BuildAffiliateLinkRequest struct {
	DestinationURL string
	CUID           string
	UTMSource      string
	UTMMedium      string
	UTMCampaign    string
	UTMTerm        string
	UTMContent     string
	BidFloor       *float64
	FallbackURL    string
}

type AffiliateLink struct {
	URL string
}

type ListTransactionsRequest struct {
	ClickDate        time.Time
	CommissionDate   time.Time
	UpdateDate       time.Time
	CampaignIDs      []int64
	MerchantGroupIDs []int64
	ProgramType      ProgramType
}

type UTMFilters struct {
	Source   []string
	Medium   []string
	Campaign []string
	Term     []string
	Content  []string
}

type GetMerchantPerformanceRequest struct {
	ClickDateStart   time.Time
	ClickDateEnd     time.Time
	CampaignIDs      []int64
	SubIDs           []string
	MerchantGroupIDs []int64
	CUIDs            []string
	PageUTM          UTMFilters
	LinkUTM          UTMFilters
	ProgramType      ProgramType
	SovrnProduct     SovrnProduct
	DeviceType       DeviceType
	Country          string
}

type ListApprovedMerchantsRequest struct {
	CampaignID   int64
	Page         int
	PageSize     int
	Names        []string
	GroupIDs     []int64
	Categories   []MerchantCategory
	Geos         []string
	ProgramTypes []ProgramType
	Domains      []string
}

// CommerceWorkflow is the bounded current Sovrn Commerce publisher surface.
type CommerceWorkflow interface {
	BuildAffiliateLink(context.Context, BuildAffiliateLinkRequest, ...socialhub.CallOption) (AffiliateLink, error)
	ListTransactions(context.Context, ListTransactionsRequest, ...socialhub.CallOption) (TransactionsResponse, error)
	GetMerchantPerformance(context.Context, GetMerchantPerformanceRequest, ...socialhub.CallOption) (MerchantPerformanceResponse, error)
	ListApprovedMerchants(context.Context, ListApprovedMerchantsRequest, ...socialhub.CallOption) (ApprovedMerchantsResponse, error)
}

type UTMInfo struct {
	Source   string `json:"utmSource"`
	Medium   string `json:"utmMedium"`
	Campaign string `json:"utmCampaign"`
	Term     string `json:"utmTerm"`
	Content  string `json:"utmContent"`
}

type Product struct {
	ID       string     `json:"productId"`
	Name     string     `json:"productName"`
	Price    ExactValue `json:"price"`
	Quantity ExactValue `json:"quantity"`
}

type Transaction struct {
	RevenueID               ExactValue      `json:"revenueId"`
	ClickSID                string          `json:"clickSid"`
	ClickID                 string          `json:"clickId,omitempty"`
	AccountID               ExactValue      `json:"accountId"`
	CommissionID            string          `json:"commissionId"`
	MerchantGroupID         ExactValue      `json:"merchantGroupId"`
	MerchantGroupName       string          `json:"merchantGroupName"`
	CampaignID              ExactValue      `json:"campaignId"`
	CampaignName            string          `json:"campaignName"`
	ClickDate               string          `json:"clickDate"`
	CommissionDate          string          `json:"commissionDate"`
	UpdateDate              string          `json:"updateDate"`
	PublisherRevenue        ExactValue      `json:"publisherRevenue"`
	OrderValue              ExactValue      `json:"orderValue"`
	CUID                    string          `json:"cuid"`
	LinkUTM                 UTMInfo         `json:"linkUtmInfo"`
	PageUTM                 UTMInfo         `json:"pageUtmInfo"`
	DestinationURL          string          `json:"destinationUrl"`
	LinkURL                 string          `json:"linkUrl,omitempty"`
	PageURL                 string          `json:"pageUrl"`
	AffiliateProduct        string          `json:"affProduct"`
	AffiliateProductSubtype string          `json:"affProductSubType"`
	ProgramType             string          `json:"programType"`
	NetworkName             string          `json:"networkName"`
	Country                 string          `json:"country"`
	DeviceType              string          `json:"deviceType"`
	Merchandise             []Product       `json:"merchandise"`
	Raw                     json.RawMessage `json:"-"`
}

func (value *Transaction) UnmarshalJSON(data []byte) error {
	type wire Transaction
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Transaction(decoded)

	var legacy struct {
		Account    json.RawMessage `json:"account"`
		Commission json.RawMessage `json:"commission"`
		Click      json.RawMessage `json:"click"`
		Merchant   json.RawMessage `json:"merchant"`
		Products   json.RawMessage `json:"product"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	if len(legacy.Account) > 0 {
		var account struct {
			AccountID    ExactValue `json:"accountId"`
			CampaignID   ExactValue `json:"campaignId"`
			CampaignName string     `json:"campaignName"`
		}
		if err := json.Unmarshal(legacy.Account, &account); err != nil {
			return err
		}
		value.AccountID, value.CampaignID, value.CampaignName = account.AccountID, account.CampaignID, account.CampaignName
	}
	if len(legacy.Commission) > 0 {
		var commission struct {
			RevenueID           ExactValue `json:"revenueId"`
			CommissionID        string     `json:"commissionId"`
			CommissionDate      string     `json:"commissionDate"`
			UpdateDate          string     `json:"updateDate"`
			OrderValue          ExactValue `json:"orderValue"`
			PublisherNetRevenue ExactValue `json:"publisherNetRevenue"`
			ProgramType         string     `json:"programType"`
		}
		if err := json.Unmarshal(legacy.Commission, &commission); err != nil {
			return err
		}
		value.RevenueID, value.CommissionID = commission.RevenueID, commission.CommissionID
		value.CommissionDate, value.UpdateDate = commission.CommissionDate, commission.UpdateDate
		value.OrderValue, value.PublisherRevenue = commission.OrderValue, commission.PublisherNetRevenue
		value.ProgramType = commission.ProgramType
	}
	if len(legacy.Click) > 0 {
		var click struct {
			ClickID      string  `json:"clickId"`
			ClickDate    string  `json:"clickDate"`
			CUID         string  `json:"cuid"`
			LinkUTM      UTMInfo `json:"linkUtmInfo"`
			PageUTM      UTMInfo `json:"pageUtmInfo"`
			PageURL      string  `json:"pageUrl"`
			LinkURL      string  `json:"linkUrl"`
			SovrnProduct string  `json:"sovrnProduct"`
			Country      string  `json:"country"`
			Device       string  `json:"device"`
		}
		if err := json.Unmarshal(legacy.Click, &click); err != nil {
			return err
		}
		value.ClickID, value.ClickDate, value.CUID = click.ClickID, click.ClickDate, click.CUID
		value.LinkUTM, value.PageUTM = click.LinkUTM, click.PageUTM
		value.PageURL, value.LinkURL = click.PageURL, click.LinkURL
		value.AffiliateProduct, value.Country, value.DeviceType = click.SovrnProduct, click.Country, click.Device
	}
	if len(legacy.Merchant) > 0 {
		var merchant struct {
			GroupID   ExactValue `json:"merchantGroupId"`
			GroupName string     `json:"merchantGroupName"`
			Network   string     `json:"network"`
		}
		if err := json.Unmarshal(legacy.Merchant, &merchant); err != nil {
			return err
		}
		value.MerchantGroupID, value.MerchantGroupName = merchant.GroupID, merchant.GroupName
		value.NetworkName = merchant.Network
	}
	if len(legacy.Products) > 0 {
		if err := json.Unmarshal(legacy.Products, &value.Merchandise); err != nil {
			return err
		}
	}
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type TransactionsResponse struct {
	Transactions []Transaction   `json:"transactions"`
	Meta         ResponseMeta    `json:"-"`
	Raw          json.RawMessage `json:"-"`
}

func (value *TransactionsResponse) UnmarshalJSON(data []byte) error {
	type wire TransactionsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = TransactionsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MerchantPerformance struct {
	Revenue           ExactValue      `json:"revenue"`
	Clicks            ExactValue      `json:"clicks"`
	Sales             ExactValue      `json:"sales"`
	Actions           ExactValue      `json:"actions"`
	ConversionRate    ExactValue      `json:"conversionRate"`
	EPC               ExactValue      `json:"epc"`
	MerchantGroupID   ExactValue      `json:"merchantGroupId"`
	MerchantGroupName string          `json:"merchantGroupName"`
	Raw               json.RawMessage `json:"-"`
}

func (value *MerchantPerformance) UnmarshalJSON(data []byte) error {
	type wire MerchantPerformance
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = MerchantPerformance(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MerchantPerformanceTotals struct {
	RevenueTotal           ExactValue `json:"revenueTotal"`
	ClicksTotal            ExactValue `json:"clicksTotal"`
	SalesTotal             ExactValue `json:"salesTotal"`
	ActionsTotal           ExactValue `json:"actionsTotal"`
	ConversionRateTotal    ExactValue `json:"conversionRateTotal"`
	EPCTotal               ExactValue `json:"epcTotal"`
	MerchantGroupIDsTotal  ExactValue `json:"merchantGroupIdsTotal"`
	MerchantGroupNameTotal ExactValue `json:"merchantGroupNameTotal"`
}

type MerchantPerformanceResponse struct {
	Data   []MerchantPerformance     `json:"data"`
	Totals MerchantPerformanceTotals `json:"totals"`
	Meta   ResponseMeta              `json:"-"`
	Raw    json.RawMessage           `json:"-"`
}

func (value *MerchantPerformanceResponse) UnmarshalJSON(data []byte) error {
	type wire MerchantPerformanceResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = MerchantPerformanceResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MerchantRate struct {
	CurrentRate ExactValue `json:"currentRate"`
	RateFormat  string     `json:"rateFormat"`
	Action      string     `json:"action"`
	Details     string     `json:"details"`
}

type CPASummary struct {
	Geo                      string         `json:"geo"`
	AverageEPC               ExactValue     `json:"averageEpc"`
	AverageOrderValue        ExactValue     `json:"averageOrderValue"`
	CalculatedCommissionRate ExactValue     `json:"calculatedCommissionRate"`
	Rates                    []MerchantRate `json:"rates"`
	Domains                  []string       `json:"domains"`
}

type CPCSummary struct {
	CalculatedEPC ExactValue `json:"calculatedEpc"`
}

type NetworkSummary struct {
	CPA []CPASummary `json:"CPA"`
	CPC CPCSummary   `json:"CPC"`
}

type ApprovedMerchant struct {
	GroupID        ExactValue         `json:"groupId"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Terms          string             `json:"terms"`
	SovrnPreferred bool               `json:"sovrnPreferred"`
	LogoImageURL   string             `json:"logoImageUrl"`
	Categories     []MerchantCategory `json:"category"`
	Sovrn          NetworkSummary     `json:"sovrn"`
	Raw            json.RawMessage    `json:"-"`
}

func (value *ApprovedMerchant) UnmarshalJSON(data []byte) error {
	type wire ApprovedMerchant
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ApprovedMerchant(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ApprovedMerchantsResponse struct {
	Results    []ApprovedMerchant `json:"results"`
	Page       int                `json:"page"`
	PerPage    int                `json:"perPage"`
	TotalItems int                `json:"totalItems"`
	Meta       ResponseMeta       `json:"-"`
	Raw        json.RawMessage    `json:"-"`
}

func (value *ApprovedMerchantsResponse) UnmarshalJSON(data []byte) error {
	type wire ApprovedMerchantsResponse
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ApprovedMerchantsResponse(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("sovrncommerce: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

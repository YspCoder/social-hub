package aliexpressaffiliate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxExactValueBytes = 256

// ExactValue preserves a provider JSON string or number without float64
// coercion. String removes JSON quoting while Bytes returns the original token.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("aliexpressaffiliate: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("aliexpressaffiliate: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("aliexpressaffiliate: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID    string
	ResponseCode string
}

type ProductType string

const (
	ProductTypeAll   ProductType = "ALL"
	ProductTypePlaza ProductType = "PLAZA"
	ProductTypeTmall ProductType = "TMALL"
)

type ProductSort string

const (
	ProductSortSalePriceAscending  ProductSort = "SALE_PRICE_ASC"
	ProductSortSalePriceDescending ProductSort = "SALE_PRICE_DESC"
	ProductSortVolumeAscending     ProductSort = "LAST_VOLUME_ASC"
	ProductSortVolumeDescending    ProductSort = "LAST_VOLUME_DESC"
)

type ProductSearchRequest struct {
	AppSignature      string
	CategoryIDs       []string
	Fields            []string
	Keywords          string
	MaximumSalePrice  uint64
	MinimumSalePrice  uint64
	PageNo            uint64
	PageSize          uint64
	ProductType       ProductType
	Sort              ProductSort
	TargetCurrency    string
	TargetLanguage    string
	TrackingID        string
	PromotionName     string
	ShipToCountry     string
	EstimatedDelivery uint64
}

type PromoCode struct {
	Code          string     `json:"promo_code"`
	CampaignType  ExactValue `json:"code_campaigntype"`
	Value         string     `json:"code_value"`
	AvailableFrom string     `json:"code_availabletime_start"`
	AvailableTo   string     `json:"code_availabletime_end"`
	MinimumSpend  ExactValue `json:"code_mini_spend"`
	Quantity      ExactValue `json:"code_quantity"`
	PromotionURL  string     `json:"code_promotionurl"`
}

type Product struct {
	SKUID                        ExactValue      `json:"sku_id"`
	TaxRate                      string          `json:"tax_rate"`
	AppSalePrice                 string          `json:"app_sale_price"`
	AppSalePriceCurrency         string          `json:"app_sale_price_currency"`
	CommissionRate               string          `json:"commission_rate"`
	EANCode                      string          `json:"ean_code"`
	Discount                     string          `json:"discount"`
	EvaluationRate               string          `json:"evaluate_rate"`
	FirstLevelCategoryID         ExactValue      `json:"first_level_category_id"`
	FirstLevelCategoryName       string          `json:"first_level_category_name"`
	LatestVolume                 ExactValue      `json:"lastest_volume"`
	HotProductCommissionRate     string          `json:"hot_product_commission_rate"`
	OriginalPrice                string          `json:"original_price"`
	OriginalPriceCurrency        string          `json:"original_price_currency"`
	PlatformProductType          string          `json:"platform_product_type"`
	ProductDetailURL             string          `json:"product_detail_url"`
	ProductID                    ExactValue      `json:"product_id"`
	ProductMainImageURL          string          `json:"product_main_image_url"`
	ProductSmallImageURLs        []string        `json:"product_small_image_urls"`
	ProductTitle                 string          `json:"product_title"`
	ProductVideoURL              string          `json:"product_video_url"`
	PromotionLink                string          `json:"promotion_link"`
	SalePrice                    string          `json:"sale_price"`
	SalePriceCurrency            string          `json:"sale_price_currency"`
	SecondLevelCategoryID        ExactValue      `json:"second_level_category_id"`
	SecondLevelCategoryName      string          `json:"second_level_category_name"`
	ShopName                     string          `json:"shop_name"`
	ShopID                       ExactValue      `json:"shop_id"`
	ShopURL                      string          `json:"shop_url"`
	TargetAppSalePrice           string          `json:"target_app_sale_price"`
	TargetAppSalePriceCurrency   string          `json:"target_app_sale_price_currency"`
	TargetOriginalPrice          string          `json:"target_original_price"`
	TargetOriginalPriceCurrency  string          `json:"target_original_price_currency"`
	TargetSalePrice              string          `json:"target_sale_price"`
	TargetSalePriceCurrency      string          `json:"target_sale_price_currency"`
	RelevantMarketCommissionRate string          `json:"relevant_market_commission_rate"`
	PromoCode                    *PromoCode      `json:"promo_code_info"`
	ShipToDays                   string          `json:"ship_to_days"`
	Raw                          json.RawMessage `json:"-"`
}

func (value *Product) UnmarshalJSON(data []byte) error {
	type wire Product
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Product(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ProductPage struct {
	Products           []Product
	CurrentPageNo      ExactValue
	CurrentRecordCount ExactValue
	TotalPageNo        ExactValue
	TotalRecordCount   ExactValue
	Meta               ResponseMeta
}

type ProductDetailRequest struct {
	AppSignature   string
	Fields         []string
	ProductIDs     []string
	TargetCurrency string
	TargetLanguage string
	TrackingID     string
	Country        string
}

type ProductDetailResult struct {
	Products           []Product
	CurrentRecordCount ExactValue
	Meta               ResponseMeta
}

type PromotionLinkType int64

const (
	PromotionLinkStandard PromotionLinkType = 0
	PromotionLinkHot      PromotionLinkType = 2
)

type LinkGenerationRequest struct {
	ShipToCountry     string
	AppSignature      string
	PromotionLinkType PromotionLinkType
	SourceValues      []string
	TrackingID        string
}

type PromotionLink struct {
	Message       string          `json:"message"`
	PromotionLink string          `json:"promotion_link"`
	SourceValue   string          `json:"source_value"`
	Raw           json.RawMessage `json:"-"`
}

func (value *PromotionLink) UnmarshalJSON(data []byte) error {
	type wire PromotionLink
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PromotionLink(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type LinkGenerationResult struct {
	Links            []PromotionLink
	TotalResultCount ExactValue
	TrackingID       string
	Meta             ResponseMeta
}

type OrderTimeType string

const (
	OrderTimePaymentCompleted      OrderTimeType = "Payment Completed Time"
	OrderTimeBuyerConfirmedReceipt OrderTimeType = "Buyer Confirmed Receipt Time"
	OrderTimeCompletedSettlement   OrderTimeType = "Completed Settlement Time"
)

type OrderStatus string

const (
	OrderStatusPaymentCompleted      OrderStatus = "Payment Completed"
	OrderStatusBuyerConfirmedReceipt OrderStatus = "Buyer Confirmed Receipt"
	OrderStatusCompletedSettlement   OrderStatus = "Completed Settlement"
	OrderStatusInvalid               OrderStatus = "Invalid"
)

type OrderListRequest struct {
	TimeType     OrderTimeType
	AppSignature string
	StartTime    time.Time
	EndTime      time.Time
	Fields       []string
	LocaleSite   string
	PageNo       uint64
	PageSize     uint64
	Status       OrderStatus
}

type Order struct {
	CompletedSettlementTime              string          `json:"completed_settlement_time"`
	OrderPlatform                        string          `json:"order_platform"`
	OrderID                              ExactValue      `json:"order_id"`
	OrderNumber                          ExactValue      `json:"order_number"`
	SubOrderID                           ExactValue      `json:"sub_order_id"`
	TrackingID                           string          `json:"tracking_id"`
	CategoryID                           ExactValue      `json:"category_id"`
	HotProduct                           string          `json:"is_hot_product"`
	EffectDetailStatus                   string          `json:"effect_detail_status"`
	IncentiveCommissionRate              string          `json:"incentive_commission_rate"`
	EstimatedIncentivePaidCommission     ExactValue      `json:"estimated_incentive_paid_commission"`
	EstimatedIncentiveFinishedCommission ExactValue      `json:"estimated_incentive_finished_commission"`
	AffiliateProduct                     string          `json:"is_affiliate_product"`
	OrderType                            string          `json:"order_type"`
	ParentOrderNumber                    ExactValue      `json:"parent_order_number"`
	Status                               string          `json:"order_status"`
	CreatedTime                          string          `json:"created_time"`
	CommissionRate                       string          `json:"commission_rate"`
	PaidAmount                           ExactValue      `json:"paid_amount"`
	PaidTime                             string          `json:"paid_time"`
	EstimatedPaidCommission              ExactValue      `json:"estimated_paid_commission"`
	FinishedAmount                       ExactValue      `json:"finished_amount"`
	FinishedTime                         string          `json:"finished_time"`
	EstimatedFinishedCommission          ExactValue      `json:"estimated_finished_commission"`
	NewBuyer                             string          `json:"is_new_buyer"`
	SettledCurrency                      string          `json:"settled_currency"`
	CustomParameters                     string          `json:"custom_parameters"`
	CustomerParameters                   string          `json:"customer_parameters"`
	NewBuyerBonusCommission              ExactValue      `json:"new_buyer_bonus_commission"`
	ProductID                            ExactValue      `json:"product_id"`
	ProductTitle                         string          `json:"product_title"`
	ProductDetailURL                     string          `json:"product_detail_url"`
	ProductMainImageURL                  string          `json:"product_main_image_url"`
	ProductCount                         ExactValue      `json:"product_count"`
	ShipToCountry                        string          `json:"ship_to_country"`
	Raw                                  json.RawMessage `json:"-"`
}

func (value *Order) UnmarshalJSON(data []byte) error {
	type wire Order
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Order(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type OrderPage struct {
	Orders             []Order
	CurrentPageNo      ExactValue
	CurrentRecordCount ExactValue
	TotalPageNo        ExactValue
	TotalRecordCount   ExactValue
	Meta               ResponseMeta
}

type OrderGetRequest struct {
	AppSignature string
	Fields       []string
	OrderIDs     []string
}

type OrderGetResult struct {
	Orders             []Order
	CurrentRecordCount ExactValue
	Meta               ResponseMeta
}

type ProductWorkflow interface {
	SearchProducts(context.Context, ProductSearchRequest, ...socialhub.CallOption) (ProductPage, error)
	GetProductDetails(context.Context, ProductDetailRequest, ...socialhub.CallOption) (ProductDetailResult, error)
}

type LinkWorkflow interface {
	GenerateLinks(context.Context, LinkGenerationRequest, ...socialhub.CallOption) (LinkGenerationResult, error)
}

type OrderWorkflow interface {
	ListOrders(context.Context, OrderListRequest, ...socialhub.CallOption) (OrderPage, error)
	GetOrders(context.Context, OrderGetRequest, ...socialhub.CallOption) (OrderGetResult, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxRequestBytes*2 || !json.Valid(trimmed) || trimmed[0] != '{' {
		return fmt.Errorf("aliexpressaffiliate: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

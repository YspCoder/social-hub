package vipunion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 1 << 20

type ResponseMeta struct {
	RequestID string
}

type GoodsSortField string

const (
	GoodsSortPrice          GoodsSortField = "PRICE"
	GoodsSortDiscount       GoodsSortField = "DISCOUNT"
	GoodsSortSales          GoodsSortField = "SALES"
	GoodsSortCommissionRate GoodsSortField = "COMMISSION_RATE"
	GoodsSortCommission     GoodsSortField = "COMMISSION"
)

type SortOrder int

const (
	SortAscending  SortOrder = 0
	SortDescending SortOrder = 1
)

type GoodsPriceScene int

const (
	GoodsPricePublicBest GoodsPriceScene = 0
	GoodsPriceUserExact  GoodsPriceScene = 1
	GoodsPricePublic     GoodsPriceScene = 2
)

type CPSInfoFlags int

const (
	CPSInfoNone        CPSInfoFlags = 0
	CPSInfoTracking    CPSInfoFlags = 1
	CPSInfoMiniProgram CPSInfoFlags = 2
	CPSInfoAll         CPSInfoFlags = 3
)

type GoodsSearchRequest struct {
	Keyword                     string
	Page                        int
	PageSize                    int
	ChanTag                     string
	OpenID                      string
	RealCall                    bool
	SortField                   GoodsSortField
	SortOrder                   *SortOrder
	PriceStart                  string
	PriceEnd                    string
	QueryReputation             *bool
	QueryStoreServiceCapability *bool
	QueryStock                  *bool
	QueryActivity               *bool
	QueryPrepay                 *bool
	QueryExclusiveCoupon        *bool
	QueryCPSInfo                *CPSInfoFlags
	Research                    *bool
	QueryFuturePrice            *bool
	ExtendSKU                   *bool
	PriceScene                  *GoodsPriceScene
	QuerySuperPriceDown         *bool
}

type GoodsLookupRequest struct {
	GoodsIDs                    []string
	ChanTag                     string
	OpenID                      string
	RealCall                    bool
	QueryDetail                 *bool
	QueryStock                  *bool
	QueryReputation             *bool
	QueryStoreServiceCapability *bool
	QueryActivity               *bool
	QueryPrepay                 *bool
	ExtendBySPU                 *bool
	SizeIDs                     map[string]string
	QueryExclusiveCoupon        *bool
	ExtendSKU                   *bool
	QueryCPSInfo                *CPSInfoFlags
	QueryFuturePrice            *bool
	PriceScene                  *GoodsPriceScene
	QuerySuperPriceDown         *bool
}

type MarketingGoodsRequest struct {
	GoodsID                     string
	ChanTag                     string
	OpenID                      string
	RealCall                    bool
	QueryDetail                 *bool
	QueryReputation             *bool
	QueryStoreServiceCapability *bool
	QueryStock                  *bool
	QueryPrepay                 *bool
	ExtendBySPU                 *bool
	SizeID                      string
	ExtendSKU                   *bool
	QueryCPSInfo                *CPSInfoFlags
	PriceScene                  *GoodsPriceScene
}

type StoreInfo struct {
	StoreID   string `json:"storeId"`
	StoreName string `json:"storeName"`
}

type GoodsCommentsInfo struct {
	Comments          int    `json:"comments"`
	GoodCommentsShare string `json:"goodCommentsShare"`
	CommentsText      string `json:"commentsText"`
}

type GoodsPromotionInfo struct {
	SalePrice         string            `json:"salePrice"`
	MarketPrice       string            `json:"marketPrice"`
	Discount          string            `json:"discount"`
	SVIP              bool              `json:"svip"`
	SalePriceDetail   string            `json:"salePriceDetail"`
	SalePriceDesc     string            `json:"salePriceDesc"`
	LowPriceTag       string            `json:"lowPriceTag"`
	AllowancePrice    string            `json:"allowancePrice"`
	NewUserSubsidyFav string            `json:"newUserSubsidyFav"`
	CouponInfos       []json.RawMessage `json:"couponInfos"`
}

type CampaignCommissionInfo struct {
	IsCampaignCommission   int    `json:"isCampaignCommission"`
	CampaignCommissionRate string `json:"campaignCommissionRate"`
	StartTime              int64  `json:"startTime"`
	EndTime                int64  `json:"endTime"`
}

type Goods struct {
	GoodsID                  string                     `json:"goodsId"`
	GoodsName                string                     `json:"goodsName"`
	ShortTitle               string                     `json:"shortTitle"`
	GoodsDescription         string                     `json:"goodsDesc"`
	DestinationURL           string                     `json:"destUrl"`
	DestinationPCURL         string                     `json:"destUrlPc"`
	GoodsThumbnailURL        string                     `json:"goodsThumbUrl"`
	GoodsMainPicture         string                     `json:"goodsMainPicture"`
	GoodsCarouselPictures    []string                   `json:"goodsCarouselPictures"`
	GoodsDetailPictures      []string                   `json:"goodsDetailPictures"`
	WhiteImage               string                     `json:"whiteImage"`
	CategoryID               int64                      `json:"categoryId"`
	CategoryName             string                     `json:"categoryName"`
	FirstCategoryID          int64                      `json:"cat1stId"`
	FirstCategoryName        string                     `json:"cat1stName"`
	SecondCategoryID         int64                      `json:"cat2ndId"`
	SecondCategoryName       string                     `json:"cat2ndName"`
	SourceType               int                        `json:"sourceType"`
	MarketPrice              string                     `json:"marketPrice"`
	VIPPrice                 string                     `json:"vipPrice"`
	EstimatePrice            string                     `json:"estimatePrice"`
	CommissionRate           string                     `json:"commissionRate"`
	Commission               string                     `json:"commission"`
	Discount                 string                     `json:"discount"`
	BrandStoreSN             string                     `json:"brandStoreSn"`
	BrandName                string                     `json:"brandName"`
	BrandLogoURL             string                     `json:"brandLogoFull"`
	SchemeStartTime          int64                      `json:"schemeStartTime"`
	SchemeEndTime            int64                      `json:"schemeEndTime"`
	SellTimeFrom             int64                      `json:"sellTimeFrom"`
	SellTimeTo               int64                      `json:"sellTimeTo"`
	Weight                   int                        `json:"weight"`
	StockStatus              int                        `json:"saleStockStatus"`
	Status                   int                        `json:"status"`
	Store                    StoreInfo                  `json:"storeInfo"`
	Comments                 GoodsCommentsInfo          `json:"commentsInfo"`
	SPUID                    string                     `json:"spuId"`
	GoodsIDsWithSameSPU      []string                   `json:"goodsIdsWithSameSpu"`
	SKUInfos                 []json.RawMessage          `json:"skuInfos"`
	CPSInfo                  map[string]json.RawMessage `json:"cpsInfo"`
	SN                       string                     `json:"sn"`
	TagNames                 []string                   `json:"tagNames"`
	ProductSales             string                     `json:"productSales"`
	AdCode                   string                     `json:"adCode"`
	Promotion                GoodsPromotionInfo         `json:"goodsPromotionInfo"`
	CampaignCommission       CampaignCommissionInfo     `json:"campaignCommissionInfo"`
	IsAllowanceGoods         int                        `json:"isAllowanceGoods"`
	Allowance                string                     `json:"allowance"`
	AllowanceStartTime       int64                      `json:"allowanceStartTime"`
	AllowanceEndTime         int64                      `json:"allowanceEndTime"`
	CanCreateGift            int                        `json:"canCreateGift"`
	SuperPriceDownFav        string                     `json:"superPriceDownFav"`
	SuperPriceDownFavPercent string                     `json:"superPriceDownFavPercent"`
	Raw                      json.RawMessage            `json:"-"`
}

func (value *Goods) UnmarshalJSON(data []byte) error {
	type wire Goods
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Goods(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type SortField struct {
	FieldName string `json:"fieldName"`
	FieldDesc string `json:"fieldDesc"`
}

type GoodsPage struct {
	Goods            []Goods
	Total            int
	Page             int
	PageSize         int
	SortFields       []SortField
	RecommendKeyword string
	BatchNumber      string
	Meta             ResponseMeta
}

type GoodsResult struct {
	Goods []Goods
	Meta  ResponseMeta
}

type MarketingGoodsResult struct {
	Goods Goods
	Meta  ResponseMeta
}

type LinkPlatform int

const (
	LinkPlatformMobile  LinkPlatform = 1
	LinkPlatformPC      LinkPlatform = 2
	LinkPlatformAndroid LinkPlatform = 3
	LinkPlatformIOS     LinkPlatform = 4
	LinkPlatformHarmony LinkPlatform = 5
)

type GoodsLandingPage int

const (
	GoodsLandingDetail       GoodsLandingPage = 1
	GoodsLandingIntermediate GoodsLandingPage = 2
)

type PromotionLinkRequest struct {
	GoodsIDs             []string
	ChanTag              string
	OpenID               string
	RealCall             bool
	StatParam            string
	EvokeQuickApp        *bool
	QueryExclusiveCoupon *bool
	GenerateShortURL     *bool
	Platform             *LinkPlatform
	AdCode               string
	RID                  string
	SizeIDs              map[string]string
	GenerateAuthorityURL *bool
	LandingPage          *GoodsLandingPage
	GiftCode             string
}

type PromotionLink struct {
	Source          string          `json:"source"`
	URL             string          `json:"url"`
	LongURL         string          `json:"longUrl"`
	UniversalURL    string          `json:"ulUrl"`
	DeepLinkURL     string          `json:"deeplinkUrl"`
	Tracking        string          `json:"traFrom"`
	NoEvokeURL      string          `json:"noEvokeUrl"`
	NoEvokeLongURL  string          `json:"noEvokeLongUrl"`
	VipWeChatURL    string          `json:"vipWxUrl"`
	VipWeChatCode   string          `json:"vipWxCode"`
	VipQuickAppURL  string          `json:"vipQuickAppUrl"`
	VipAlipayURL    string          `json:"vipZfbUrl"`
	VipAlipayScheme string          `json:"vipZfbSchemeUrl"`
	VipAlipayHTTPS  string          `json:"vipZfbHttpsUrl"`
	Command         string          `json:"onlyCommand"`
	Tips            string          `json:"tips"`
	Raw             json.RawMessage `json:"-"`
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

type PromotionLinkResult struct {
	Links []PromotionLink
	Meta  ResponseMeta
}

type OrderStatus int16

const (
	OrderStatusInvalid   OrderStatus = 0
	OrderStatusPending   OrderStatus = 1
	OrderStatusCompleted OrderStatus = 2
)

type OrderListRequest struct {
	Status                  *OrderStatus
	OrderTimeStart          time.Time
	OrderTimeEnd            time.Time
	UpdateTimeStart         time.Time
	UpdateTimeEnd           time.Time
	OrderSNs                []string
	Page                    int
	PageSize                int
	ChanTag                 string
	QuerySubsidyActivity    *bool
	FilterSplitParentOrders *bool
}

type AfterSaleDetail struct {
	ChangedCommission string `json:"afterSaleChangedCommission"`
	ChangedGoodsCount int    `json:"afterSaleChangedGoodsCount"`
	AfterSaleSN       string `json:"afterSaleSn"`
	Status            int    `json:"afterSaleStatus"`
	Type              int    `json:"afterSaleType"`
	FinishTime        int64  `json:"afterSaleFinishTime"`
	NewOrderSN        string `json:"newOrderSn"`
}

type OrderDetail struct {
	GoodsID                          string            `json:"goodsId"`
	GoodsName                        string            `json:"goodsName"`
	GoodsThumbnailURL                string            `json:"goodsThumb"`
	GoodsCount                       int               `json:"goodsCount"`
	CommissionTotalCost              string            `json:"commissionTotalCost"`
	CommissionRate                   string            `json:"commissionRate"`
	Commission                       string            `json:"commission"`
	CommissionCode                   string            `json:"commCode"`
	CommissionName                   string            `json:"commName"`
	OrderSource                      string            `json:"orderSource"`
	AfterSales                       []AfterSaleDetail `json:"afterSaleInfo"`
	SizeID                           string            `json:"sizeId"`
	Status                           int16             `json:"status"`
	BrandStoreSN                     string            `json:"brandStoreSn"`
	BrandStoreName                   string            `json:"brandStoreName"`
	SPUID                            string            `json:"spuId"`
	GoodsFinalPrice                  string            `json:"goodsFinalPrice"`
	IsSubsidyTaskOrder               bool              `json:"isSubsidyTaskOrder"`
	SubsidyTaskOrderStatus           int               `json:"subsidyTaskOrderStatus"`
	SubsidyTaskGoodsAward            string            `json:"subsidyTaskGoodsAward"`
	FirstCategoryCode                string            `json:"cat1Code"`
	FirstCategoryName                string            `json:"cat1Name"`
	SecondCategoryCode               string            `json:"cat2Code"`
	SecondCategoryName               string            `json:"cat2Name"`
	ThirdCategoryCode                string            `json:"cat3Code"`
	ThirdCategoryName                string            `json:"cat3Name"`
	BaseCommissionSettleRate         string            `json:"baseCommissionSettleRate"`
	DeductionCommission              string            `json:"DeductionCommission"`
	EstimateCommissionAfterDeduction string            `json:"estimateCommissionAfterDeduction"`
	IsCampaignCommission             int               `json:"isCampaignCommission"`
	CampaignCommissionAmount         string            `json:"campaignCommissionAmt"`
	CampaignCommissionStatus         int16             `json:"campaignCommissionStatus"`
	GiftGoodsDiscount                string            `json:"giftGoodsFav"`
}

type Order struct {
	OrderSN                          string          `json:"orderSn"`
	Status                           int16           `json:"status"`
	NewCustomer                      int16           `json:"newCustomer"`
	ChannelTag                       string          `json:"channelTag"`
	OrderTime                        int64           `json:"orderTime"`
	SignTime                         int64           `json:"signTime"`
	SettledTime                      int64           `json:"settledTime"`
	Details                          []OrderDetail   `json:"detailList"`
	LastUpdateTime                   int64           `json:"lastUpdateTime"`
	Settled                          int16           `json:"settled"`
	SelfBuy                          int             `json:"selfBuy"`
	OrderSubStatusName               string          `json:"orderSubStatusName"`
	Commission                       string          `json:"commission"`
	AfterSaleChangeCommission        string          `json:"afterSaleChangeCommission"`
	AfterSaleChangeGoodsCount        int             `json:"afterSaleChangeGoodsCount"`
	CommissionEnterTime              int64           `json:"commissionEnterTime"`
	OrderSource                      string          `json:"orderSource"`
	PID                              string          `json:"pid"`
	IsPrepay                         int             `json:"isPrepay"`
	StatParam                        string          `json:"statParam"`
	IsSplit                          int             `json:"isSplit"`
	ParentSN                         string          `json:"parentSn"`
	OrderTrackReason                 int             `json:"orderTrackReason"`
	AppKey                           string          `json:"appKey"`
	TotalCost                        string          `json:"totalCost"`
	OpenID                           string          `json:"openId"`
	AdCode                           string          `json:"adCode"`
	BlackOrder                       int             `json:"fdsBlackOrder"`
	OrderQuality                     string          `json:"orderQuality"`
	BaseCommissionSettleRate         string          `json:"baseCommissionSettleRate"`
	DeductionCommission              string          `json:"DeductionCommission"`
	EstimateCommissionAfterDeduction string          `json:"estimateCommissionAfterDeduction"`
	OrderType                        int16           `json:"orderType"`
	GiftCode                         string          `json:"giftCode"`
	GiftAmount                       string          `json:"giftAmount"`
	GiftReturnFlag                   int             `json:"giftReturnFlag"`
	GiftReturnAmount                 string          `json:"giftReturnAmount"`
	GiftTypeName                     string          `json:"giftTypeName"`
	Raw                              json.RawMessage `json:"-"`
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
	Orders   []Order
	Total    int
	Page     int
	PageSize int
	Meta     ResponseMeta
}

type GoodsWorkflow interface {
	SearchGoods(context.Context, GoodsSearchRequest, ...socialhub.CallOption) (GoodsPage, error)
	GetGoods(context.Context, GoodsLookupRequest, ...socialhub.CallOption) (GoodsResult, error)
	GetMarketingGoods(context.Context, MarketingGoodsRequest, ...socialhub.CallOption) (MarketingGoodsResult, error)
}

type LinkWorkflow interface {
	GeneratePromotionLinks(context.Context, PromotionLinkRequest, ...socialhub.CallOption) (PromotionLinkResult, error)
}

type OrderWorkflow interface {
	ListOrders(context.Context, OrderListRequest, ...socialhub.CallOption) (OrderPage, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return fmt.Errorf("vipunion: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

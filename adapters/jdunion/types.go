package jdunion

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
	maxProviderObjectBytes = 2 << 20
)

// ExactValue preserves a JD JSON string or number without float64 coercion.
// String removes JSON quoting while Bytes returns the original JSON token.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("jdunion: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("jdunion: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("jdunion: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
}

type GoodsField string

const (
	GoodsFieldVideoInfo               GoodsField = "videoInfo"
	GoodsFieldHotWords                GoodsField = "hotWords"
	GoodsFieldSimilar                 GoodsField = "similar"
	GoodsFieldDocumentInfo            GoodsField = "documentInfo"
	GoodsFieldSKULabelInfo            GoodsField = "skuLabelInfo"
	GoodsFieldPromotionLabelInfo      GoodsField = "promotionLabelInfo"
	GoodsFieldCompanyType             GoodsField = "companyType"
	GoodsFieldSeckillSpecialPriceInfo GoodsField = "seckillSpecialPriceInfo"
)

type GoodsQueryRequest struct {
	EliteID   uint64
	PageIndex uint64
	PageSize  uint64
	SortName  string
	Sort      string
	PID       string
	Fields    []GoodsField
}

type PriceInfo struct {
	Price             ExactValue `json:"price"`
	LowestPrice       ExactValue `json:"lowestPrice"`
	LowestCouponPrice ExactValue `json:"lowestCouponPrice"`
	LowestPriceType   ExactValue `json:"lowestPriceType"`
	HistoryPriceDay   ExactValue `json:"historyPriceDay"`
}

type CommissionInfo struct {
	Commission               ExactValue `json:"commission"`
	CommissionShare          ExactValue `json:"commissionShare"`
	CouponCommission         ExactValue `json:"couponCommission"`
	PlusCommissionShare      ExactValue `json:"plusCommissionShare"`
	Locked                   ExactValue `json:"isLock"`
	StartTime                ExactValue `json:"startTime"`
	EndTime                  ExactValue `json:"endTime"`
	CommissionStockMaximum   ExactValue `json:"lhCommissionStockMax"`
	CommissionStockRemaining ExactValue `json:"lhCommissionStockRemaining"`
}

type CategoryInfo struct {
	LevelOneID     ExactValue `json:"cid1"`
	LevelOneName   string     `json:"cid1Name"`
	LevelTwoID     ExactValue `json:"cid2"`
	LevelTwoName   string     `json:"cid2Name"`
	LevelThreeID   ExactValue `json:"cid3"`
	LevelThreeName string     `json:"cid3Name"`
}

type ShopInfo struct {
	ShopID            ExactValue `json:"shopId"`
	ShopName          string     `json:"shopName"`
	ShopLevel         ExactValue `json:"shopLevel"`
	UserEvaluateScore ExactValue `json:"userEvaluateScore"`
	LogisticsScore    ExactValue `json:"logisticsLvyueScore"`
	AfterServiceScore ExactValue `json:"afterServiceScore"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ImageInfo struct {
	WhiteImage string
	Images     []ImageURL
	Raw        json.RawMessage
}

func (value *ImageInfo) UnmarshalJSON(data []byte) error {
	var wire struct {
		WhiteImage string          `json:"whiteImage"`
		ImageList  json.RawMessage `json:"imageList"`
	}
	if err := decodeProviderObject(data, &wire); err != nil {
		return err
	}
	images, err := decodeProviderList[ImageURL](wire.ImageList, "urlInfo")
	if err != nil {
		return err
	}
	value.WhiteImage, value.Images = wire.WhiteImage, images
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Coupon struct {
	BindType     ExactValue `json:"bindType"`
	Discount     ExactValue `json:"discount"`
	Link         string     `json:"link"`
	PlatformType ExactValue `json:"platformType"`
	Quota        ExactValue `json:"quota"`
	GetStartTime ExactValue `json:"getStartTime"`
	GetEndTime   ExactValue `json:"getEndTime"`
	UseStartTime ExactValue `json:"useStartTime"`
	UseEndTime   ExactValue `json:"useEndTime"`
	Status       ExactValue `json:"couponStatus"`
	Best         ExactValue `json:"isBest"`
	HotValue     ExactValue `json:"hotValue"`
}

type CouponInfo struct {
	Coupons []Coupon
	Raw     json.RawMessage
}

func (value *CouponInfo) UnmarshalJSON(data []byte) error {
	var wire struct {
		CouponList json.RawMessage `json:"couponList"`
	}
	if err := decodeProviderObject(data, &wire); err != nil {
		return err
	}
	coupons, err := decodeProviderList[Coupon](wire.CouponList, "coupon")
	if err != nil {
		return err
	}
	value.Coupons = coupons
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ResourceInfo struct {
	EliteID   ExactValue `json:"eliteId"`
	EliteName string     `json:"eliteName"`
}

type DocumentInfo struct {
	Document string `json:"document"`
	Discount string `json:"discount"`
}

type Goods struct {
	SKUID                 ExactValue      `json:"skuId"`
	ItemID                ExactValue      `json:"itemId"`
	SPUID                 ExactValue      `json:"spuid"`
	SKUName               string          `json:"skuName"`
	MaterialURL           string          `json:"materialUrl"`
	Owner                 string          `json:"owner"`
	BrandCode             ExactValue      `json:"brandCode"`
	BrandName             string          `json:"brandName"`
	Comments              ExactValue      `json:"comments"`
	GoodCommentsShare     ExactValue      `json:"goodCommentsShare"`
	InOrderCount30Days    ExactValue      `json:"inOrderCount30Days"`
	InOrderCount30DaysSKU ExactValue      `json:"inOrderCount30DaysSku"`
	Price                 *PriceInfo      `json:"priceInfo"`
	Commission            *CommissionInfo `json:"commissionInfo"`
	Category              *CategoryInfo   `json:"categoryInfo"`
	Shop                  *ShopInfo       `json:"shopInfo"`
	Images                *ImageInfo      `json:"imageInfo"`
	Coupons               *CouponInfo     `json:"couponInfo"`
	Resource              *ResourceInfo   `json:"resourceInfo"`
	Document              *DocumentInfo   `json:"documentInfo"`
	Raw                   json.RawMessage `json:"-"`
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

type GoodsPage struct {
	Goods      []Goods
	TotalCount ExactValue
	Meta       ResponseMeta
}

type PromotionScene uint64

const (
	PromotionSceneAffiliate PromotionScene = 1
	PromotionSceneMainSite  PromotionScene = 2
)

type PromotionRequest struct {
	MaterialID      string
	SiteID          string
	PositionID      uint64
	SubUnionID      string
	ExternalID      string
	PID             string
	CouponURL       string
	GiftCouponKey   string
	ChannelID       uint64
	RID             string
	GenerateCommand bool
	SceneID         PromotionScene
}

type Promotion struct {
	ClickURL string
	JCommand string
	Meta     ResponseMeta
	Raw      json.RawMessage
}

type OrderQueryType uint64

const (
	OrderQueryCreated   OrderQueryType = 1
	OrderQueryCompleted OrderQueryType = 2
	OrderQueryUpdated   OrderQueryType = 3
)

type OrderField string

const (
	OrderFieldGoodsInfo    OrderField = "goodsInfo"
	OrderFieldCategoryInfo OrderField = "categoryInfo"
	OrderFieldKeyword      OrderField = "keyword"
)

type OrderQueryRequest struct {
	PageIndex    uint64
	PageSize     uint64
	QueryType    OrderQueryType
	StartTime    time.Time
	EndTime      time.Time
	ChildUnionID uint64
	Key          string
	Fields       []OrderField
	OrderID      uint64
}

type OrderGoodsInfo struct {
	Owner     string     `json:"owner"`
	MainSKUID ExactValue `json:"mainSkuId"`
	ProductID ExactValue `json:"productId"`
	ImageURL  string     `json:"imageUrl"`
	ShopName  string     `json:"shopName"`
	ShopID    ExactValue `json:"shopId"`
}

type OrderSecretInfo struct {
	Keyword  string     `json:"secretKeyWord"`
	SecretID ExactValue `json:"secretId"`
}

type Order struct {
	OrderID          ExactValue       `json:"orderId"`
	OrderRowID       ExactValue       `json:"id"`
	SKUID            ExactValue       `json:"skuId"`
	ItemID           ExactValue       `json:"itemId"`
	CallerItemID     ExactValue       `json:"callerItemId"`
	SKUName          string           `json:"skuName"`
	SKUCount         ExactValue       `json:"skuNum"`
	SKUReturnCount   ExactValue       `json:"skuReturnNum"`
	SKUFrozenCount   ExactValue       `json:"skuFrozenNum"`
	Price            ExactValue       `json:"price"`
	EstimatedCost    ExactValue       `json:"estimateCosPrice"`
	ActualCost       ExactValue       `json:"actualCosPrice"`
	EstimatedFee     ExactValue       `json:"estimateFee"`
	ActualFee        ExactValue       `json:"actualFee"`
	CommissionRate   ExactValue       `json:"commissionRate"`
	FinalRate        ExactValue       `json:"finalRate"`
	SubsidyRate      ExactValue       `json:"subsidyRate"`
	SubSideRate      ExactValue       `json:"subSideRate"`
	PromotionAmount  ExactValue       `json:"proPriceAmount"`
	GiftCouponAmount ExactValue       `json:"giftCouponOcsAmount"`
	ValidCode        ExactValue       `json:"validCode"`
	OrderTime        string           `json:"orderTime"`
	OrderPaidTime    string           `json:"orderPayTime"`
	FinishTime       string           `json:"finishTime"`
	ModifiedTime     string           `json:"modifyTime"`
	UnionID          ExactValue       `json:"unionId"`
	PID              string           `json:"pid"`
	SiteID           ExactValue       `json:"siteId"`
	PositionID       ExactValue       `json:"positionId"`
	ChannelID        ExactValue       `json:"channelId"`
	SubUnionID       string           `json:"subUnionId"`
	ExternalID       string           `json:"ext1"`
	RID              string           `json:"rid"`
	GiftCouponKey    string           `json:"giftCouponKey"`
	UnionRole        ExactValue       `json:"unionRole"`
	UnionAlias       string           `json:"unionAlias"`
	ParentID         ExactValue       `json:"parentId"`
	SubCPUnionID     ExactValue       `json:"subCpUnionId"`
	Sign             string           `json:"sign"`
	Category         *CategoryInfo    `json:"categoryInfo"`
	Goods            *OrderGoodsInfo  `json:"goodsInfo"`
	Secret           *OrderSecretInfo `json:"secretInfo"`
	Raw              json.RawMessage  `json:"-"`
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
	Orders  []Order
	HasMore bool
	Meta    ResponseMeta
}

type GoodsWorkflow interface {
	QueryJingfen(context.Context, GoodsQueryRequest, ...socialhub.CallOption) (GoodsPage, error)
}

type PromotionWorkflow interface {
	CreatePromotion(context.Context, PromotionRequest, ...socialhub.CallOption) (*Promotion, error)
}

type OrderWorkflow interface {
	QueryOrderRows(context.Context, OrderQueryRequest, ...socialhub.CallOption) (OrderPage, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return fmt.Errorf("jdunion: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

func decodeProviderList[T any](data json.RawMessage, wrapper string) ([]T, error) {
	trimmed, err := decodeObjectOrString(data)
	if err != nil {
		return nil, err
	}
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var values []T
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return nil, err
		}
		return values, nil
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("jdunion: provider list must be an object or array")
	}
	if wrapper != "" {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		if nested, found := object[wrapper]; found {
			return decodeProviderList[T](nested, "")
		}
	}
	var value T
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, err
	}
	return []T{value}, nil
}

func decodeObjectOrString(data json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) {
		return nil, fmt.Errorf("jdunion: invalid provider JSON")
	}
	if trimmed[0] != '"' {
		return append([]byte(nil), trimmed...), nil
	}
	var encoded string
	if err := json.Unmarshal(trimmed, &encoded); err != nil {
		return nil, err
	}
	decoded := bytes.TrimSpace([]byte(encoded))
	if len(decoded) == 0 || len(decoded) > maxProviderObjectBytes || !json.Valid(decoded) {
		return nil, fmt.Errorf("jdunion: invalid JSON encoded in provider string")
	}
	return decoded, nil
}

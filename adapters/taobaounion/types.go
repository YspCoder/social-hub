package taobaounion

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

// ExactValue preserves a TOP JSON string or number without float64 coercion.
// String removes JSON quoting while Bytes returns the original JSON token.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("taobaounion: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("taobaounion: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("taobaounion: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

type ResponseMeta struct {
	RequestID string
}

type LinkPlatform int64

const (
	LinkPlatformPC       LinkPlatform = 1
	LinkPlatformWireless LinkPlatform = 2
)

type MaterialSearchRequest struct {
	AdzoneID            string
	Query               string
	CategoryIDs         []string
	MaterialID          int64
	PageNo              int64
	PageSize            int64
	Platform            LinkPlatform
	StartPrice          int64
	EndPrice            int64
	StartCommissionRate int64
	EndCommissionRate   int64
	HasCoupon           *bool
	IsTmall             *bool
	IsOverseas          *bool
	Sort                string
	RelationID          string
	SpecialID           string
	PageResultKey       string
	BizSceneID          string
	PromotionType       string
}

type Material struct {
	ItemID               ExactValue      `json:"item_id"`
	Title                string          `json:"title"`
	ShortTitle           string          `json:"short_title"`
	Description          string          `json:"item_description"`
	PictureURL           string          `json:"pict_url"`
	WhiteImageURL        string          `json:"white_image"`
	SmallImageURLs       []string        `json:"small_images"`
	ItemURL              string          `json:"item_url"`
	PromotionURL         string          `json:"url"`
	CouponShareURL       string          `json:"coupon_share_url"`
	ReservePrice         ExactValue      `json:"reserve_price"`
	FinalPrice           ExactValue      `json:"zk_final_price"`
	SalePrice            ExactValue      `json:"sale_price"`
	CommissionRate       ExactValue      `json:"commission_rate"`
	CommissionType       string          `json:"commission_type"`
	CouponID             string          `json:"coupon_id"`
	CouponDescription    string          `json:"coupon_info"`
	CouponAmount         ExactValue      `json:"coupon_amount"`
	CouponStartFee       ExactValue      `json:"coupon_start_fee"`
	CouponStartTime      string          `json:"coupon_start_time"`
	CouponEndTime        string          `json:"coupon_end_time"`
	CouponTotalCount     ExactValue      `json:"coupon_total_count"`
	CouponRemainingCount ExactValue      `json:"coupon_remain_count"`
	ShopTitle            string          `json:"shop_title"`
	SellerNickname       string          `json:"nick"`
	SellerID             ExactValue      `json:"seller_id"`
	SellerType           ExactValue      `json:"user_type"`
	CategoryID           ExactValue      `json:"category_id"`
	CategoryName         string          `json:"category_name"`
	LevelOneCategoryID   ExactValue      `json:"level_one_category_id"`
	LevelOneCategoryName string          `json:"level_one_category_name"`
	Volume               ExactValue      `json:"volume"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *Material) UnmarshalJSON(data []byte) error {
	type wire Material
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Material(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MaterialSearchResult struct {
	Materials     []Material
	TotalResults  ExactValue
	PageResultKey string
	Meta          ResponseMeta
}

type ItemInfoRequest struct {
	ItemIDs         []string
	Platform        LinkPlatform
	IP              string
	BizSceneID      string
	PromotionType   string
	RelationID      string
	ManageItemPubID string
}

type Item struct {
	ItemID               ExactValue      `json:"num_iid"`
	InputItemID          ExactValue      `json:"input_num_iid"`
	Title                string          `json:"title"`
	PictureURL           string          `json:"pict_url"`
	SmallImageURLs       []string        `json:"small_images"`
	ItemURL              string          `json:"item_url"`
	ReservePrice         ExactValue      `json:"reserve_price"`
	FinalPrice           ExactValue      `json:"zk_final_price"`
	SalePrice            ExactValue      `json:"sale_price"`
	ProvinceCity         string          `json:"provcity"`
	ShopNickname         string          `json:"nick"`
	SellerID             ExactValue      `json:"seller_id"`
	SellerType           ExactValue      `json:"user_type"`
	LevelOneCategoryName string          `json:"cat_name"`
	LeafCategoryName     string          `json:"cat_leaf_name"`
	MaterialLibraryType  string          `json:"material_lib_type"`
	Volume               ExactValue      `json:"volume"`
	ShopDSR              ExactValue      `json:"shop_dsr"`
	FreeShipment         bool            `json:"free_shipment"`
	ConsumerProtection   bool            `json:"is_prepay"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *Item) UnmarshalJSON(data []byte) error {
	type wire Item
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Item(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ItemInfoResult struct {
	Items []Item
	Meta  ResponseMeta
}

type LinkItemInput struct {
	ItemID            string `json:"item_id"`
	CouponID          string `json:"coupon_id,omitempty"`
	ExternalID        string `json:"external_id,omitempty"`
	GeneralPlan       string `json:"dx,omitempty"`
	SKUID             int64  `json:"sku_id,omitempty"`
	TargetCoupon      int64  `json:"is_target_coupon,omitempty"`
	ManagePublisherID int64  `json:"manage_pub_id,omitempty"`
}

type LinkMaterialInput struct {
	MaterialURL  string `json:"material_url"`
	CouponID     string `json:"coupon_id,omitempty"`
	TargetCoupon int64  `json:"is_target_coupon,omitempty"`
}

type LinkConversionRequest struct {
	AdzoneID      string
	Items         []LinkItemInput
	Materials     []LinkMaterialInput
	BizSceneID    string
	PromotionType string
	RelationID    string
	SpecialID     string
}

type PromotionInfo struct {
	CommissionRate         ExactValue `json:"commission_rate"`
	CommissionType         string     `json:"commission_type"`
	PromotionPrice         ExactValue `json:"promotion_price"`
	MultipleItemPriceCount ExactValue `json:"multiple_items_prices_count"`
}

type CouponInfo struct {
	ActivityID     string     `json:"activity_id"`
	Amount         ExactValue `json:"coupon_amount"`
	Description    string     `json:"coupon_desc"`
	StartTime      string     `json:"coupon_start_time"`
	EndTime        string     `json:"coupon_end_time"`
	RemainingCount ExactValue `json:"coupon_remain_count"`
	Type           ExactValue `json:"coupon_type"`
}

type ConvertedLink struct {
	ItemID              ExactValue `json:"item_id"`
	MaterialID          ExactValue `json:"material_id"`
	MaterialType        ExactValue `json:"material_type"`
	CPSLongURL          string     `json:"cps_long_url"`
	CPSShortURL         string     `json:"cps_short_url"`
	CPSShortPassword    string     `json:"cps_short_tpwd"`
	CPSFullPassword     string     `json:"cps_full_tpwd"`
	CouponLongURL       string     `json:"coupon_long_url"`
	CouponShortURL      string     `json:"coupon_short_url"`
	CouponShortPassword string     `json:"coupon_short_tpwd"`
	CouponFullPassword  string     `json:"coupon_full_tpwd"`
	ShareUnitURL        string     `json:"share_unit_url"`
	TaoBrickURL         string     `json:"tao_brick_url"`
	OriginalPasswordURL string     `json:"tpwd_origin_url"`
}

type ItemLinkResult struct {
	InputItemID ExactValue      `json:"input_item_id"`
	Code        ExactValue      `json:"code"`
	Message     string          `json:"msg"`
	ExtraInfo   string          `json:"extra_info"`
	Link        *ConvertedLink  `json:"link_info_dto"`
	Promotion   *PromotionInfo  `json:"promotion_info_dto"`
	Coupon      *CouponInfo     `json:"coupon_info_dto"`
	Raw         json.RawMessage `json:"-"`
}

func (value *ItemLinkResult) UnmarshalJSON(data []byte) error {
	type wire ItemLinkResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = ItemLinkResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type MaterialLinkResult struct {
	InputMaterialURL string          `json:"input_material_url"`
	Code             ExactValue      `json:"code"`
	Message          string          `json:"msg"`
	ExtraInfo        string          `json:"extra_info"`
	Link             *ConvertedLink  `json:"link_info_dto"`
	Promotion        *PromotionInfo  `json:"promotion_info_dto"`
	Coupon           *CouponInfo     `json:"coupon_info_dto"`
	Raw              json.RawMessage `json:"-"`
}

func (value *MaterialLinkResult) UnmarshalJSON(data []byte) error {
	type wire MaterialLinkResult
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = MaterialLinkResult(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type LinkConversionResult struct {
	ItemLinks       []ItemLinkResult
	MaterialLinks   []MaterialLinkResult
	ShopLinks       []json.RawMessage
	EventLinks      []json.RawMessage
	BusinessError   ExactValue
	BusinessMessage string
	Meta            ResponseMeta
}

type TaoPasswordRequest struct {
	URL string
}

type TaoPassword struct {
	PasswordSimple string       `json:"password_simple"`
	Model          string       `json:"model"`
	Meta           ResponseMeta `json:"-"`
}

type OrderQueryType int64

const (
	OrderQueryCreated OrderQueryType = 1
	OrderQueryPaid    OrderQueryType = 2
	OrderQuerySettled OrderQueryType = 3
	OrderQueryUpdated OrderQueryType = 4
)

type OrderMemberType int64

const (
	OrderMemberSecondParty OrderMemberType = 2
	OrderMemberThirdParty  OrderMemberType = 3
)

type OrderStatus int64

const (
	OrderStatusSettled  OrderStatus = 3
	OrderStatusCreated  OrderStatus = 11
	OrderStatusPaid     OrderStatus = 12
	OrderStatusClosed   OrderStatus = 13
	OrderStatusReceived OrderStatus = 14
)

type OrderScene int64

const (
	OrderSceneAll     OrderScene = 1
	OrderSceneChannel OrderScene = 2
	OrderSceneMember  OrderScene = 3
)

type PageDirection int64

const (
	PagePrevious PageDirection = -1
	PageNext     PageDirection = 1
)

type OrderDetailsRequest struct {
	StartTime     time.Time
	EndTime       time.Time
	QueryType     OrderQueryType
	PageSize      int64
	PageNo        int64
	PositionIndex string
	Direction     PageDirection
	MemberType    OrderMemberType
	Status        OrderStatus
	Scene         OrderScene
}

type Order struct {
	TradeID           ExactValue      `json:"trade_id"`
	ParentTradeID     ExactValue      `json:"trade_parent_id"`
	ThirdPartyOrderID ExactValue      `json:"tp_order_id"`
	ItemID            ExactValue      `json:"item_id"`
	ItemTitle         string          `json:"item_title"`
	ItemImageURL      string          `json:"item_img"`
	ItemURL           string          `json:"item_link"`
	ItemCount         ExactValue      `json:"item_num"`
	ItemPrice         ExactValue      `json:"item_price"`
	PaidAmount        ExactValue      `json:"alipay_total_price"`
	SettlementAmount  ExactValue      `json:"pay_price"`
	CommissionRate    ExactValue      `json:"total_commission_rate"`
	CommissionAmount  ExactValue      `json:"total_commission_fee"`
	EstimatedShare    ExactValue      `json:"pub_share_pre_fee"`
	SettledShare      ExactValue      `json:"pub_share_fee"`
	Status            ExactValue      `json:"tk_status"`
	OrderType         string          `json:"order_type"`
	FlowSource        string          `json:"flow_source"`
	TerminalType      string          `json:"terminal_type"`
	SellerNickname    string          `json:"seller_nick"`
	SellerShopTitle   string          `json:"seller_shop_title"`
	AdzoneID          ExactValue      `json:"adzone_id"`
	SiteID            ExactValue      `json:"site_id"`
	PublisherID       ExactValue      `json:"pub_id"`
	RelationID        ExactValue      `json:"relation_id"`
	SpecialID         ExactValue      `json:"special_id"`
	CreatedTime       string          `json:"tk_create_time"`
	PaidTime          string          `json:"tk_paid_time"`
	PlatformPaidTime  string          `json:"tb_paid_time"`
	SettlementTime    string          `json:"tk_earning_time"`
	ModifiedTime      string          `json:"modified_time"`
	ClickTime         string          `json:"click_time"`
	Raw               json.RawMessage `json:"-"`
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
	Orders        []Order
	PositionIndex string
	PageNo        ExactValue
	PageSize      ExactValue
	PreviousPage  ExactValue
	NextPage      ExactValue
	HasPrevious   bool
	HasNext       bool
	Meta          ResponseMeta
}

type MaterialWorkflow interface {
	SearchMaterials(context.Context, MaterialSearchRequest, ...socialhub.CallOption) (MaterialSearchResult, error)
}

type ItemWorkflow interface {
	GetItems(context.Context, ItemInfoRequest, ...socialhub.CallOption) (ItemInfoResult, error)
}

type LinkWorkflow interface {
	ConvertLinks(context.Context, LinkConversionRequest, ...socialhub.CallOption) (LinkConversionResult, error)
	CreateTaoPassword(context.Context, TaoPasswordRequest, ...socialhub.CallOption) (*TaoPassword, error)
}

type OrderWorkflow interface {
	ListOrders(context.Context, OrderDetailsRequest, ...socialhub.CallOption) (OrderPage, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return fmt.Errorf("taobaounion: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

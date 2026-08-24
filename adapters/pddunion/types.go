package pddunion

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

// ExactValue preserves a Pinduoduo JSON string or number without float64
// coercion. String removes JSON quoting while Bytes returns the original token.
type ExactValue struct {
	raw json.RawMessage
}

func (value *ExactValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxExactValueBytes || !json.Valid(trimmed) {
		return fmt.Errorf("pddunion: invalid exact value")
	}
	first := trimmed[0]
	if first != '"' && first != '-' && (first < '0' || first > '9') && !bytes.Equal(trimmed, []byte("null")) {
		return fmt.Errorf("pddunion: exact value must be a JSON string, number, or null")
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
		return fmt.Errorf("pddunion: decode target and exact value are required")
	}
	return json.Unmarshal(value.raw, target)
}

// StringList accepts both JSON arrays and provider strings containing a JSON
// array. A non-JSON string is retained as a single value.
type StringList []string

func (value *StringList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) {
		return fmt.Errorf("pddunion: invalid string list")
	}
	if bytes.Equal(trimmed, []byte("null")) {
		*value = nil
		return nil
	}
	if trimmed[0] == '[' {
		return json.Unmarshal(trimmed, (*[]string)(value))
	}
	if trimmed[0] != '"' {
		return fmt.Errorf("pddunion: string list must be an array or string")
	}
	var encoded string
	if err := json.Unmarshal(trimmed, &encoded); err != nil {
		return err
	}
	nested := bytes.TrimSpace([]byte(encoded))
	if len(nested) > 0 && nested[0] == '[' && json.Valid(nested) {
		return json.Unmarshal(nested, (*[]string)(value))
	}
	if encoded == "" {
		*value = nil
	} else {
		*value = []string{encoded}
	}
	return nil
}

type ResponseMeta struct {
	RequestID string
}

type RecommendationChannel int64

const (
	RecommendationChannelBudget RecommendationChannel = 0
	RecommendationChannelHot    RecommendationChannel = 1
	RecommendationChannelBrand  RecommendationChannel = 2
	RecommendationChannelMall   RecommendationChannel = 3
)

type GoodsRecommendRequest struct {
	Offset  int64
	Limit   int64
	Channel *RecommendationChannel
}

type GoodsDetailRequest struct {
	GoodsSign        string
	PID              string
	CustomParameters string
	SearchID         string
}

type Goods struct {
	GoodsSign              string          `json:"goods_sign"`
	GoodsID                ExactValue      `json:"goods_id"`
	GoodsName              string          `json:"goods_name"`
	GoodsDescription       string          `json:"goods_desc"`
	GoodsImageURL          string          `json:"goods_image_url"`
	GoodsThumbnailURL      string          `json:"goods_thumbnail_url"`
	GoodsGalleryURLs       StringList      `json:"goods_gallery_urls"`
	VideoURLs              json.RawMessage `json:"video_urls"`
	CategoryID             ExactValue      `json:"category_id"`
	CategoryName           string          `json:"category_name"`
	CategoryIDs            []ExactValue    `json:"cat_ids"`
	OptionID               ExactValue      `json:"opt_id"`
	OptionIDs              []ExactValue    `json:"opt_ids"`
	OptionName             string          `json:"opt_name"`
	MallID                 ExactValue      `json:"mall_id"`
	MallName               string          `json:"mall_name"`
	MerchantType           ExactValue      `json:"merchant_type"`
	MinimumGroupPrice      ExactValue      `json:"min_group_price"`
	MinimumNormalPrice     ExactValue      `json:"min_normal_price"`
	HasCoupon              bool            `json:"has_coupon"`
	CouponDiscount         ExactValue      `json:"coupon_discount"`
	CouponMinimumOrder     ExactValue      `json:"coupon_min_order_amount"`
	CouponRemaining        ExactValue      `json:"coupon_remain_quantity"`
	CouponTotal            ExactValue      `json:"coupon_total_quantity"`
	CouponStartTime        ExactValue      `json:"coupon_start_time"`
	CouponEndTime          ExactValue      `json:"coupon_end_time"`
	PromotionRate          ExactValue      `json:"promotion_rate"`
	PredictedPromotionRate ExactValue      `json:"predict_promotion_rate"`
	ShareRate              ExactValue      `json:"share_rate"`
	SoldQuantity           ExactValue      `json:"sold_quantity"`
	SalesTip               string          `json:"sales_tip"`
	EvaluationCount        ExactValue      `json:"goods_eval_count"`
	SearchID               string          `json:"search_id"`
	PlanType               ExactValue      `json:"plan_type"`
	OnlySceneAuthorized    bool            `json:"only_scene_auth"`
	Raw                    json.RawMessage `json:"-"`
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
	Goods    []Goods
	ListID   string
	SearchID string
	Total    ExactValue
	Meta     ResponseMeta
}

type GoodsDetailResult struct {
	Goods []Goods
	Meta  ResponseMeta
}

type PromotionLinkRequest struct {
	PID                     string
	GoodsSign               string
	GenerateShortURL        bool
	MultiGroup              *bool
	CustomParameters        string
	PullNew                 *bool
	GenerateWeAppWebView    *bool
	GenerateWeApp           *bool
	GenerateQQApp           *bool
	GenerateSchemaURL       *bool
	GenerateWeiboAppWebView *bool
	SearchID                string
}

type MiniProgramInfo struct {
	AppID             string `json:"app_id"`
	UserName          string `json:"user_name"`
	PagePath          string `json:"page_path"`
	Title             string `json:"title"`
	Description       string `json:"desc"`
	SourceDisplayName string `json:"source_display_name"`
	IconURL           string `json:"we_app_icon_url"`
	BannerURL         string `json:"banner_url"`
}

type QQMiniProgramInfo struct {
	AppID             string `json:"app_id"`
	UserName          string `json:"user_name"`
	PagePath          string `json:"page_path"`
	Title             string `json:"title"`
	Description       string `json:"desc"`
	SourceDisplayName string `json:"source_display_name"`
	IconURL           string `json:"qq_app_icon_url"`
	BannerURL         string `json:"banner_url"`
}

type PromotionLink struct {
	URL                     string             `json:"url"`
	ShortURL                string             `json:"short_url"`
	MobileURL               string             `json:"mobile_url"`
	MobileShortURL          string             `json:"mobile_short_url"`
	WeAppWebViewURL         string             `json:"we_app_web_view_url"`
	WeAppWebViewShortURL    string             `json:"we_app_web_view_short_url"`
	SchemaURL               string             `json:"schema_url"`
	WeiboAppWebViewURL      string             `json:"weibo_app_web_view_url"`
	WeiboAppWebViewShortURL string             `json:"weibo_app_web_view_short_url"`
	FirefoxOpenAppURL       string             `json:"ffx_open_app_url"`
	WeApp                   *MiniProgramInfo   `json:"we_app_info"`
	QQApp                   *QQMiniProgramInfo `json:"qq_app_info"`
	Raw                     json.RawMessage    `json:"-"`
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

type OrderIncrementRequest struct {
	StartUpdateTime time.Time
	EndUpdateTime   time.Time
	PageSize        int64
	Page            int64
}

type Order struct {
	OrderID                string          `json:"order_id"`
	OrderSN                string          `json:"order_sn"`
	GoodsSign              string          `json:"goods_sign"`
	GoodsID                ExactValue      `json:"goods_id"`
	GoodsName              string          `json:"goods_name"`
	GoodsThumbnailURL      string          `json:"goods_thumbnail_url"`
	GoodsQuantity          ExactValue      `json:"goods_quantity"`
	GoodsPrice             ExactValue      `json:"goods_price"`
	OrderAmount            ExactValue      `json:"order_amount"`
	PromotionRate          ExactValue      `json:"promotion_rate"`
	PromotionAmount        ExactValue      `json:"promotion_amount"`
	PlatformDiscount       ExactValue      `json:"platform_discount"`
	CPAReward              ExactValue      `json:"cpa_new"`
	OrderStatus            ExactValue      `json:"order_status"`
	OrderStatusDescription string          `json:"order_status_desc"`
	FailureReason          string          `json:"fail_reason"`
	OrderType              ExactValue      `json:"type"`
	Direct                 ExactValue      `json:"is_direct"`
	OrderCreateTime        ExactValue      `json:"order_create_time"`
	OrderPayTime           ExactValue      `json:"order_pay_time"`
	GroupSuccessTime       ExactValue      `json:"order_group_success_time"`
	OrderReceiveTime       ExactValue      `json:"order_receive_time"`
	OrderVerifyTime        ExactValue      `json:"order_verify_time"`
	OrderSettleTime        ExactValue      `json:"order_settle_time"`
	OrderModifiedTime      ExactValue      `json:"order_modify_at"`
	GroupID                ExactValue      `json:"group_id"`
	PID                    string          `json:"p_id"`
	AuthorizedDuoID        ExactValue      `json:"auth_duo_id"`
	InvestmentDuoID        ExactValue      `json:"zs_duo_id"`
	BatchNumber            string          `json:"batch_no"`
	CustomParameters       string          `json:"custom_parameters"`
	CategoryIDs            []ExactValue    `json:"cat_ids"`
	MallName               string          `json:"mall_name"`
	Raw                    json.RawMessage `json:"-"`
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
	Orders     []Order
	TotalCount ExactValue
	Meta       ResponseMeta
}

type GoodsWorkflow interface {
	RecommendGoods(context.Context, GoodsRecommendRequest, ...socialhub.CallOption) (GoodsPage, error)
	GetGoods(context.Context, GoodsDetailRequest, ...socialhub.CallOption) (GoodsDetailResult, error)
}

type LinkWorkflow interface {
	GeneratePromotionLinks(context.Context, PromotionLinkRequest, ...socialhub.CallOption) (PromotionLinkResult, error)
}

type OrderWorkflow interface {
	ListIncrementalOrders(context.Context, OrderIncrementRequest, ...socialhub.CallOption) (OrderPage, error)
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || !json.Valid(trimmed) || trimmed[0] != '{' {
		return fmt.Errorf("pddunion: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

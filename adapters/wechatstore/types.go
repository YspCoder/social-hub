package wechatstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

// StableAccessToken is a sensitive server credential and its local expiry.
type StableAccessToken struct {
	Value     string
	ExpiresAt time.Time
	ExpiresIn time.Duration
}

func (StableAccessToken) String() string   { return "wechatstore.StableAccessToken{REDACTED}" }
func (StableAccessToken) GoString() string { return "wechatstore.StableAccessToken{REDACTED}" }

type ShopSubjectType string

const (
	SubjectEnterprise         ShopSubjectType = "企业"
	SubjectIndividualBusiness ShopSubjectType = "个体工商户"
)

type ShopStatus string

const (
	ShopStatusOpening       ShopStatus = "opening"
	ShopStatusOpenFinished  ShopStatus = "open_finished"
	ShopStatusClosing       ShopStatus = "closing"
	ShopStatusCloseFinished ShopStatus = "close_finished"
)

// StoreInfo contains the non-customer store fields documented by the basic
// information endpoint. It deliberately has no raw response field.
type StoreInfo struct {
	Nickname      string
	HeadImageURL  string
	SubjectType   ShopSubjectType
	Status        ShopStatus
	Username      string
	IsLocalLife   bool
	OpenTimestamp int64
}

func (StoreInfo) String() string   { return "wechatstore.StoreInfo{REDACTED}" }
func (StoreInfo) GoString() string { return "wechatstore.StoreInfo{REDACTED}" }

// CatalogID preserves provider identifiers without float64 precision loss.
type CatalogID string

func (value CatalogID) String() string { return string(value) }

func (value CatalogID) MarshalJSON() ([]byte, error) {
	if !validCatalogID(string(value)) {
		return nil, fmt.Errorf("wechatstore: catalog ID is invalid")
	}
	return json.Marshal(string(value))
}

func (value *CatalogID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var text string
	if len(trimmed) != 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return fmt.Errorf("wechatstore: invalid catalog ID")
		}
	} else {
		text = string(trimmed)
	}
	if !validCatalogID(text) {
		return fmt.Errorf("wechatstore: catalog ID must be a decimal string")
	}
	*value = CatalogID(text)
	return nil
}

type ProductDataType int

const (
	ProductDataOnline ProductDataType = 1
	ProductDataDraft  ProductDataType = 2
	ProductDataBoth   ProductDataType = 3
)

type ProductListStatus int

const (
	ProductListStatusInitial    ProductListStatus = 0
	ProductListStatusListed     ProductListStatus = 5
	ProductListStatusRecycleBin ProductListStatus = 6
	ProductListStatusDelisted   ProductListStatus = 11
)

type ProductStatus int

const (
	ProductStatusInitial          ProductStatus = 0
	ProductStatusListed           ProductStatus = 5
	ProductStatusRecycleBin       ProductStatus = 6
	ProductStatusMerchantDelisted ProductStatus = 11
	ProductStatusSoldOut          ProductStatus = 12
	ProductStatusPolicyDelisted   ProductStatus = 13
	ProductStatusDepositDelisted  ProductStatus = 14
	ProductStatusBrandExpired     ProductStatus = 15
	ProductStatusBanned           ProductStatus = 20
)

type ProductEditStatus int

const (
	ProductEditStatusInitial           ProductEditStatus = 0
	ProductEditStatusEditing           ProductEditStatus = 1
	ProductEditStatusUnderReview       ProductEditStatus = 2
	ProductEditStatusRejected          ProductEditStatus = 3
	ProductEditStatusApproved          ProductEditStatus = 4
	ProductEditStatusUploading         ProductEditStatus = 7
	ProductEditStatusUploadFailed      ProductEditStatus = 8
	ProductEditStatusSubmitting        ProductEditStatus = 70
	ProductEditStatusQuotaFailed       ProductEditStatus = 72
	ProductEditStatusRateLimitedFailed ProductEditStatus = 73
)

type ProductSubStatus int

const (
	ProductSubStatusDefault         ProductSubStatus = 0
	ProductSubStatusSupplierRemoved ProductSubStatus = 1
	ProductSubStatusBrandRestricted ProductSubStatus = 2
)

type SKUStatus int

const (
	SKUStatusInitial    SKUStatus = 0
	SKUStatusListed     SKUStatus = 5
	SKUStatusDelisted   SKUStatus = 11
	SKUStatusPresaleEnd SKUStatus = 21
)

type GetProductRequest struct {
	ProductID CatalogID
	DataType  ProductDataType
}

type ListProductsRequest struct {
	Status   *ProductListStatus
	PageSize int
	NextKey  string
}

type CategoryRef struct {
	CategoryID CatalogID `json:"cat_id"`
}

type ProductAttribute struct {
	Key   string `json:"attr_key"`
	Value string `json:"attr_value"`
}

func (ProductAttribute) String() string   { return "wechatstore.ProductAttribute{REDACTED}" }
func (ProductAttribute) GoString() string { return "wechatstore.ProductAttribute{REDACTED}" }

type ProductDescription struct {
	Images []string `json:"imgs"`
	Text   string   `json:"desc"`
}

func (ProductDescription) String() string   { return "wechatstore.ProductDescription{REDACTED}" }
func (ProductDescription) GoString() string { return "wechatstore.ProductDescription{REDACTED}" }

type SKU struct {
	SKUID        CatalogID          `json:"sku_id"`
	ExternalID   string             `json:"out_sku_id"`
	ThumbnailURL string             `json:"thumb_img"`
	SalePrice    int64              `json:"sale_price"`
	Stock        int64              `json:"stock_num"`
	Code         string             `json:"sku_code"`
	Attributes   []ProductAttribute `json:"sku_attrs"`
	Status       SKUStatus          `json:"status"`
	Barcode      string             `json:"bar_code"`
}

func (SKU) String() string   { return "wechatstore.SKU{REDACTED}" }
func (SKU) GoString() string { return "wechatstore.SKU{REDACTED}" }

// Product exposes stable catalog fields only. The full provider object is not
// retained because it may contain commercially sensitive product content.
type Product struct {
	ProductID        CatalogID          `json:"product_id"`
	ExternalID       string             `json:"out_product_id"`
	Title            string             `json:"title"`
	ShortTitle       string             `json:"short_title"`
	HeadImages       []string           `json:"head_imgs"`
	Description      ProductDescription `json:"desc_info"`
	Status           ProductStatus      `json:"status"`
	EditStatus       ProductEditStatus  `json:"edit_status"`
	SubStatus        ProductSubStatus   `json:"sub_status"`
	MinimumPrice     int64              `json:"min_price"`
	LegacyCategories []CategoryRef      `json:"cats"`
	Categories       []CategoryRef      `json:"cats_v2"`
	Attributes       []ProductAttribute `json:"attrs"`
	SKUs             []SKU              `json:"skus"`
	SPUCode          string             `json:"spu_code"`
	ProductType      int                `json:"product_type"`
	TotalSold        int64              `json:"total_sold_num"`
}

func (Product) String() string   { return "wechatstore.Product{REDACTED}" }
func (Product) GoString() string { return "wechatstore.Product{REDACTED}" }

type ProductDetail struct {
	Online *Product
	Draft  *Product
}

func (ProductDetail) String() string   { return "wechatstore.ProductDetail{REDACTED}" }
func (ProductDetail) GoString() string { return "wechatstore.ProductDetail{REDACTED}" }

type ProductPage struct {
	ProductIDs []CatalogID
	NextKey    string
	Total      int64
}

func (ProductPage) String() string   { return "wechatstore.ProductPage{REDACTED}" }
func (ProductPage) GoString() string { return "wechatstore.ProductPage{REDACTED}" }

type CredentialsWorkflow interface {
	GetStableAccessToken(context.Context, ...socialhub.CallOption) (*StableAccessToken, error)
	ForceRefreshStableAccessToken(context.Context, ...socialhub.CallOption) (*StableAccessToken, error)
}

type StoreWorkflow interface {
	GetInfo(context.Context, ...socialhub.CallOption) (*StoreInfo, error)
}

type CatalogWorkflow interface {
	ListProducts(context.Context, ListProductsRequest, ...socialhub.CallOption) (*ProductPage, error)
	GetProduct(context.Context, GetProductRequest, ...socialhub.CallOption) (*ProductDetail, error)
}

var (
	_ CredentialsWorkflow = (*Client)(nil)
	_ StoreWorkflow       = (*Client)(nil)
	_ CatalogWorkflow     = (*Client)(nil)
)

package aliexpressaffiliate

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	productQueryMethod  = "aliexpress.affiliate.product.query"
	productDetailMethod = "aliexpress.affiliate.productdetail.get"
)

func (client *Client) SearchProducts(
	ctx context.Context,
	input ProductSearchRequest,
	options ...socialhub.CallOption,
) (ProductPage, error) {
	const operation = "search_products"
	trackingID, err := client.trackingID(operation, input.TrackingID, false)
	if err != nil {
		return ProductPage{}, err
	}
	if !validProductSearch(input) {
		return ProductPage{}, invalidArgument(operation, "product filters, fields, target locale, or pagination are invalid")
	}
	values := make(url.Values)
	setString(values, "app_signature", client.appSignature(input.AppSignature))
	if len(input.CategoryIDs) > 0 {
		values.Set("category_ids", strings.Join(input.CategoryIDs, ","))
	}
	if len(input.Fields) > 0 {
		values.Set("fields", strings.Join(input.Fields, ","))
	}
	setString(values, "keywords", input.Keywords)
	setUint(values, "max_sale_price", input.MaximumSalePrice)
	setUint(values, "min_sale_price", input.MinimumSalePrice)
	setUint(values, "page_no", input.PageNo)
	setUint(values, "page_size", input.PageSize)
	setString(values, "platform_product_type", string(input.ProductType))
	setString(values, "sort", string(input.Sort))
	setString(values, "target_currency", input.TargetCurrency)
	setString(values, "target_language", input.TargetLanguage)
	setString(values, "tracking_id", trackingID)
	setString(values, "promotion_name", input.PromotionName)
	setString(values, "ship_to_country", input.ShipToCountry)
	setUint(values, "delivery_days", input.EstimatedDelivery)
	var response struct {
		CurrentPageNo      ExactValue `json:"current_page_no"`
		CurrentRecordCount ExactValue `json:"current_record_count"`
		Products           []Product  `json:"products"`
		TotalPageNo        ExactValue `json:"total_page_no"`
		TotalRecordCount   ExactValue `json:"total_record_count"`
	}
	meta, err := client.doForm(ctx, operation, productQueryMethod, values, &response, options...)
	if err != nil {
		return ProductPage{}, err
	}
	return ProductPage{
		Products: response.Products, CurrentPageNo: response.CurrentPageNo,
		CurrentRecordCount: response.CurrentRecordCount, TotalPageNo: response.TotalPageNo,
		TotalRecordCount: response.TotalRecordCount, Meta: meta,
	}, nil
}

func (client *Client) GetProductDetails(
	ctx context.Context,
	input ProductDetailRequest,
	options ...socialhub.CallOption,
) (ProductDetailResult, error) {
	const operation = "get_product_details"
	trackingID, err := client.trackingID(operation, input.TrackingID, false)
	if err != nil {
		return ProductDetailResult{}, err
	}
	if !validProductDetails(input) {
		return ProductDetailResult{}, invalidArgument(operation, "product IDs, fields, or target locale are invalid")
	}
	values := make(url.Values)
	setString(values, "app_signature", client.appSignature(input.AppSignature))
	if len(input.Fields) > 0 {
		values.Set("fields", strings.Join(input.Fields, ","))
	}
	values.Set("product_ids", strings.Join(input.ProductIDs, ","))
	setString(values, "target_currency", input.TargetCurrency)
	setString(values, "target_language", input.TargetLanguage)
	setString(values, "tracking_id", trackingID)
	setString(values, "country", input.Country)
	var response struct {
		CurrentRecordCount ExactValue `json:"current_record_count"`
		Products           []Product  `json:"products"`
	}
	meta, err := client.doForm(ctx, operation, productDetailMethod, values, &response, options...)
	if err != nil {
		return ProductDetailResult{}, err
	}
	return ProductDetailResult{
		Products: response.Products, CurrentRecordCount: response.CurrentRecordCount, Meta: meta,
	}, nil
}

func validProductSearch(input ProductSearchRequest) bool {
	if !validOptionalText(input.Keywords, 1024) || !validFields(input.Fields) ||
		!validProviderLong(input.MaximumSalePrice) || !validProviderLong(input.MinimumSalePrice) ||
		input.MaximumSalePrice > 0 && input.MinimumSalePrice > input.MaximumSalePrice ||
		!validProviderLong(input.PageNo) || input.PageSize > 50 ||
		!validProductType(input.ProductType) || !validProductSort(input.Sort) ||
		!validCurrency(input.TargetCurrency, false) || !validLanguage(input.TargetLanguage) ||
		input.TrackingID != "" && !validCSVValue(input.TrackingID, 512) ||
		!validOptionalText(input.PromotionName, 512) || !validCountry(input.ShipToCountry) ||
		!validDeliveryDays(input.EstimatedDelivery) || input.AppSignature != "" && !validOpaque(input.AppSignature, 4096) {
		return false
	}
	seen := make(map[string]struct{}, len(input.CategoryIDs))
	for _, categoryID := range input.CategoryIDs {
		if !validNumericID(categoryID, 32) {
			return false
		}
		if _, duplicate := seen[categoryID]; duplicate {
			return false
		}
		seen[categoryID] = struct{}{}
	}
	return true
}

func validProductDetails(input ProductDetailRequest) bool {
	return validStringIDs(input.ProductIDs) && validFields(input.Fields) &&
		validCurrency(input.TargetCurrency, true) && validLanguage(input.TargetLanguage) &&
		(input.TrackingID == "" || validCSVValue(input.TrackingID, 512)) && validCountry(input.Country) &&
		(input.AppSignature == "" || validOpaque(input.AppSignature, 4096))
}

func validProductType(value ProductType) bool {
	return value == "" || value == ProductTypeAll || value == ProductTypePlaza || value == ProductTypeTmall
}

func validProductSort(value ProductSort) bool {
	switch value {
	case "", ProductSortSalePriceAscending, ProductSortSalePriceDescending, ProductSortVolumeAscending, ProductSortVolumeDescending:
		return true
	default:
		return false
	}
}

func validDeliveryDays(value uint64) bool {
	switch value {
	case 0, 3, 5, 7, 10:
		return true
	default:
		return false
	}
}

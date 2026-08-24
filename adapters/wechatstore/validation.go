package wechatstore

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	DefaultProductPageSize   = 10
	MaximumProductPageSize   = 30
	maxAppIDLength           = 256
	maxSecretReferenceLength = 4_096
	maxCredentialLength      = 16_384
	maxCatalogIDLength       = 128
	maxCursorLength          = 4_096
	maxRequestIDLength       = 256
	maxRequestBodyBytes      = 64 << 10
)

func validSensitive(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalSensitive(value string, maximum int) bool {
	return value == "" || validSensitive(value, maximum)
}

func validCatalogID(value string) bool {
	if value == "" || len(value) > maxCatalogIDLength {
		return false
	}
	nonzero := false
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
		nonzero = nonzero || character != '0'
	}
	return nonzero
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "idempotency keys are not supported by this read-only operation")
	}
	if len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "field selection is not supported by this WeChat Store operation")
	}
	if !validOptionalSensitive(resolved.RequestID, maxRequestIDLength) {
		return socialhub.CallOptions{}, invalidArgument(operation, "request ID is invalid")
	}
	return resolved, nil
}

func normalizeGetProductRequest(input GetProductRequest) (GetProductRequest, error) {
	if !validCatalogID(string(input.ProductID)) {
		return GetProductRequest{}, invalidArgument("get_product", "product ID is required and invalid")
	}
	if input.DataType == 0 {
		input.DataType = ProductDataOnline
	}
	if input.DataType != ProductDataOnline && input.DataType != ProductDataDraft && input.DataType != ProductDataBoth {
		return GetProductRequest{}, invalidArgument("get_product", "data type must be online, draft, or both")
	}
	return input, nil
}

func normalizeListProductsRequest(input ListProductsRequest) (ListProductsRequest, error) {
	if input.PageSize == 0 {
		input.PageSize = DefaultProductPageSize
	}
	if input.PageSize < 1 || input.PageSize > MaximumProductPageSize {
		return ListProductsRequest{}, invalidArgument("list_products", "page size must be between 1 and 30")
	}
	if !validOptionalSensitive(input.NextKey, maxCursorLength) {
		return ListProductsRequest{}, invalidArgument("list_products", "next key is invalid")
	}
	if input.Status != nil {
		status := *input.Status
		if status != ProductListStatusInitial && status != ProductListStatusListed &&
			status != ProductListStatusRecycleBin && status != ProductListStatusDelisted {
			return ListProductsRequest{}, invalidArgument("list_products", "product status filter is invalid")
		}
		input.Status = &status
	}
	return input, nil
}

func validShopStatus(value ShopStatus) bool {
	return value == ShopStatusOpening || value == ShopStatusOpenFinished ||
		value == ShopStatusClosing || value == ShopStatusCloseFinished
}

func validSubjectType(value ShopSubjectType) bool {
	return value == SubjectEnterprise || value == SubjectIndividualBusiness
}

func validOptionalHTTPSURL(value string) bool {
	if value == "" {
		return true
	}
	if !validSensitive(value, 4_096) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func validateStoreInfo(info storeInfoWire) error {
	if !validSensitive(info.Nickname, 1_024) || !validOptionalHTTPSURL(info.HeadImageURL) ||
		!validSubjectType(info.SubjectType) || !validShopStatus(info.Status) ||
		!validSensitive(info.Username, 512) || (info.IsLocalLife != 0 && info.IsLocalLife != 1) ||
		info.OpenTimestamp < 0 {
		return platformContractError("get_store_info", "WeChat returned an invalid store information response")
	}
	return nil
}

func validateProductDetail(detail productDetailResponse, request GetProductRequest) error {
	if detail.Product == nil && detail.EditProduct == nil {
		return platformContractError("get_product", "WeChat returned no product data")
	}
	if request.DataType == ProductDataOnline && detail.Product == nil {
		return platformContractError("get_product", "WeChat returned no online product data")
	}
	if request.DataType == ProductDataDraft && detail.EditProduct == nil {
		return platformContractError("get_product", "WeChat returned no draft product data")
	}
	for _, product := range []*Product{detail.Product, detail.EditProduct} {
		if product != nil && product.ProductID != request.ProductID {
			return platformContractError("get_product", "WeChat returned a mismatched product identifier")
		}
	}
	return nil
}

func validateProductPage(page listProductsResponse) error {
	if page.Total < 0 || len(page.ProductIDs) > MaximumProductPageSize ||
		!validOptionalSensitive(page.NextKey, maxCursorLength) {
		return platformContractError("list_products", "WeChat returned an invalid product page")
	}
	for _, productID := range page.ProductIDs {
		if !validCatalogID(string(productID)) {
			return platformContractError("list_products", "WeChat returned an invalid product identifier")
		}
	}
	return nil
}

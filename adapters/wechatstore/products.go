package wechatstore

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

type listProductsRequest struct {
	Status   *ProductListStatus `json:"status,omitempty"`
	PageSize int                `json:"page_size"`
	NextKey  string             `json:"next_key,omitempty"`
}

type listProductsResponse struct {
	ProductIDs []CatalogID `json:"product_ids"`
	NextKey    string      `json:"next_key"`
	Total      int64       `json:"total_num"`
}

type getProductRequest struct {
	ProductID CatalogID       `json:"product_id"`
	DataType  ProductDataType `json:"data_type"`
}

type productDetailResponse struct {
	Product     *Product `json:"product"`
	EditProduct *Product `json:"edit_product"`
}

// ListProducts returns product IDs using WeChat's next_key cursor contract.
func (client *Client) ListProducts(ctx context.Context, input ListProductsRequest, options ...socialhub.CallOption) (*ProductPage, error) {
	const operation = "list_products"
	normalized, err := normalizeListProductsRequest(input)
	if err != nil {
		return nil, err
	}
	accessToken, err := client.accessToken(ctx, options...)
	if err != nil {
		return nil, err
	}
	query := url.Values{"access_token": {accessToken}}
	payload := listProductsRequest{
		Status: normalized.Status, PageSize: normalized.PageSize, NextKey: normalized.NextKey,
	}
	var response listProductsResponse
	if err := client.doJSON(ctx, operation, http.MethodPost, "/channels/ec/product/list/get", query, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := validateProductPage(response); err != nil {
		return nil, err
	}
	return &ProductPage{
		ProductIDs: append([]CatalogID(nil), response.ProductIDs...),
		NextKey:    response.NextKey, Total: response.Total,
	}, nil
}

// GetProduct reads the online version, draft version, or both for one product.
func (client *Client) GetProduct(ctx context.Context, input GetProductRequest, options ...socialhub.CallOption) (*ProductDetail, error) {
	const operation = "get_product"
	normalized, err := normalizeGetProductRequest(input)
	if err != nil {
		return nil, err
	}
	accessToken, err := client.accessToken(ctx, options...)
	if err != nil {
		return nil, err
	}
	query := url.Values{"access_token": {accessToken}}
	payload := getProductRequest{ProductID: normalized.ProductID, DataType: normalized.DataType}
	var response productDetailResponse
	if err := client.doJSON(ctx, operation, http.MethodPost, "/channels/ec/product/get", query, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := validateProductDetail(response, normalized); err != nil {
		return nil, err
	}
	return &ProductDetail{Online: response.Product, Draft: response.EditProduct}, nil
}

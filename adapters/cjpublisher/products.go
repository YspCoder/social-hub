package cjpublisher

import (
	"context"

	"social-hub/pkg/socialhub"
)

const searchProductFeedsQuery = `
query SearchCJProductFeeds(
  $companyId: ID!, $feedType: FeedType, $partnerIds: [ID!],
  $advertiserCountry: String, $offset: Int, $limit: Int
) {
  productFeeds(
    companyId: $companyId, feedType: $feedType, partnerIds: $partnerIds,
    advertiserCountry: $advertiserCountry, offset: $offset, limit: $limit
  ) {
    totalCount
    limit
    count
    resultList {
      adId
      advertiserId
      advertiserName
      advertiserCountry
      sourceFeedType
      currency
      language
      feedName
      lastUpdated
      productCount
    }
  }
}`

const productFields = `
    totalCount
    limit
    count
    nextPage
    resultList {
      id
      adId
      advertiserId
      advertiserName
      advertiserCountry
      brand
      catalogId
      description
      title
      imageLink
      additionalImageLink
      isDeleted
      itemListId
      itemListName
      lastUpdated
      link
      mobileLink
      price { amount currency }
      salePrice { amount currency }
      salePriceEffectiveDateStart
      salePriceEffectiveDateEnd
      serviceableAreas
      shipping {
        price { amount currency }
        service
        locationGroupName
        region
        postalCode
        locationId
        country
        minimumHandlingTime
        maximumHandlingTime
        minimumTransitTime
        maximumTransitTime
      }
      sourceFeedType
      targetCountry
`

const searchProductsQuery = `
query SearchCJProducts(
  $companyId: ID!, $adIds: [ID!], $keywords: [String!], $partnerIds: [ID!],
  $partnerStatus: PartnerStatus, $excludePartnerIds: [ID!], $productIds: [ID!],
  $excludeProductIds: [ID!], $advertiserCountries: [String!], $highPrice: Float,
  $lowPrice: Float, $currency: String, $itemListIds: [ID!],
  $includeDeletedProducts: Boolean, $serviceableAreas: [String!],
  $excludeServiceableAreas: [String!], $availability: Availability,
  $sortBy: SortBy, $sortOrder: SortOrder, $offset: Int, $limit: Int, $page: String
) {
  products(
    companyId: $companyId, adIds: $adIds, keywords: $keywords, partnerIds: $partnerIds,
    partnerStatus: $partnerStatus, excludePartnerIds: $excludePartnerIds,
    productIds: $productIds, excludeProductIds: $excludeProductIds,
    advertiserCountries: $advertiserCountries, highPrice: $highPrice, lowPrice: $lowPrice,
    currency: $currency, itemListIds: $itemListIds,
    includeDeletedProducts: $includeDeletedProducts, serviceableAreas: $serviceableAreas,
    excludeServiceableAreas: $excludeServiceableAreas, availability: $availability,
    sortBy: $sortBy, sortOrder: $sortOrder, offset: $offset, limit: $limit, page: $page
  ) {` + productFields + `
    }
  }
}`

const searchProductsWithLinkCodeQuery = `
query SearchCJProductsWithLinkCode(
  $companyId: ID!, $adIds: [ID!], $keywords: [String!], $partnerIds: [ID!],
  $partnerStatus: PartnerStatus, $excludePartnerIds: [ID!], $productIds: [ID!],
  $excludeProductIds: [ID!], $advertiserCountries: [String!], $highPrice: Float,
  $lowPrice: Float, $currency: String, $itemListIds: [ID!],
  $includeDeletedProducts: Boolean, $serviceableAreas: [String!],
  $excludeServiceableAreas: [String!], $availability: Availability,
  $sortBy: SortBy, $sortOrder: SortOrder, $offset: Int, $limit: Int, $page: String,
  $pid: ID!, $shopperId: ID
) {
  products(
    companyId: $companyId, adIds: $adIds, keywords: $keywords, partnerIds: $partnerIds,
    partnerStatus: $partnerStatus, excludePartnerIds: $excludePartnerIds,
    productIds: $productIds, excludeProductIds: $excludeProductIds,
    advertiserCountries: $advertiserCountries, highPrice: $highPrice, lowPrice: $lowPrice,
    currency: $currency, itemListIds: $itemListIds,
    includeDeletedProducts: $includeDeletedProducts, serviceableAreas: $serviceableAreas,
    excludeServiceableAreas: $excludeServiceableAreas, availability: $availability,
    sortBy: $sortBy, sortOrder: $sortOrder, offset: $offset, limit: $limit, page: $page
  ) {` + productFields + `
      linkCode(pid: $pid, shopperId: $shopperId) { html clickUrl imageUrl }
    }
  }
}`

func (client *Client) SearchProductFeeds(
	ctx context.Context,
	input SearchProductFeedsRequest,
	options ...socialhub.CallOption,
) (ProductFeedsResponse, error) {
	const operation = "search_product_feeds"
	if !validSearchProductFeeds(input) {
		return ProductFeedsResponse{}, invalidArgument(operation, "feed type, partner IDs, country, offset, or limit is invalid")
	}
	variables := map[string]any{"companyId": client.publisherID}
	setVariable(variables, "feedType", string(input.FeedType))
	setStringSliceVariable(variables, "partnerIds", input.PartnerIDs)
	setVariable(variables, "advertiserCountry", input.AdvertiserCountry)
	setPositiveIntVariable(variables, "offset", input.Offset)
	setPositiveIntVariable(variables, "limit", input.Limit)
	var data struct {
		ProductFeeds ProductFeedsResponse `json:"productFeeds"`
	}
	meta, raw, err := client.doGraphQL(ctx, client.productsAPI, operation, searchProductFeedsQuery, variables, &data, options...)
	data.ProductFeeds.Meta, data.ProductFeeds.Raw = meta, raw
	if err == nil {
		err = validateProductFeedsResponse(operation, data.ProductFeeds, input.Limit)
	}
	return data.ProductFeeds, err
}

func (client *Client) SearchProducts(
	ctx context.Context,
	input SearchProductsRequest,
	options ...socialhub.CallOption,
) (ProductsResponse, error) {
	const operation = "search_products"
	propertyID := resolvePropertyID(input.PromotionalPropertyID, client.websiteID)
	if !validSearchProducts(input, propertyID) {
		return ProductsResponse{}, invalidArgument(operation, "product filters, pagination, price range, enum, or link-code PID is invalid")
	}
	variables := map[string]any{"companyId": client.publisherID}
	setStringSliceVariable(variables, "adIds", input.AdIDs)
	setStringSliceVariable(variables, "keywords", input.Keywords)
	setStringSliceVariable(variables, "partnerIds", input.PartnerIDs)
	setVariable(variables, "partnerStatus", string(input.PartnerStatus))
	setStringSliceVariable(variables, "excludePartnerIds", input.ExcludePartnerIDs)
	setStringSliceVariable(variables, "productIds", input.ProductIDs)
	setStringSliceVariable(variables, "excludeProductIds", input.ExcludeProductIDs)
	setStringSliceVariable(variables, "advertiserCountries", input.AdvertiserCountries)
	setOptionalFloat(variables, "highPrice", input.HighPrice)
	setOptionalFloat(variables, "lowPrice", input.LowPrice)
	setVariable(variables, "currency", input.Currency)
	setStringSliceVariable(variables, "itemListIds", input.ItemListIDs)
	setOptionalBool(variables, "includeDeletedProducts", input.IncludeDeletedProducts)
	setStringSliceVariable(variables, "serviceableAreas", input.ServiceableAreas)
	setStringSliceVariable(variables, "excludeServiceableAreas", input.ExcludeServiceableAreas)
	setVariable(variables, "availability", string(input.Availability))
	setVariable(variables, "sortBy", string(input.SortBy))
	setVariable(variables, "sortOrder", string(input.SortOrder))
	setPositiveIntVariable(variables, "offset", input.Offset)
	setPositiveIntVariable(variables, "limit", input.Limit)
	setVariable(variables, "page", input.Page)
	query := searchProductsQuery
	if input.IncludeLinkCode {
		query = searchProductsWithLinkCodeQuery
		variables["pid"] = propertyID
		setVariable(variables, "shopperId", input.ShopperID)
	}
	var data struct {
		Products ProductsResponse `json:"products"`
	}
	meta, raw, err := client.doGraphQL(ctx, client.productsAPI, operation, query, variables, &data, options...)
	data.Products.Meta, data.Products.Raw = meta, raw
	if err == nil {
		err = validateProductsResponse(operation, data.Products, input.Limit)
	}
	return data.Products, err
}

func setVariable(variables map[string]any, key, value string) {
	if value != "" {
		variables[key] = value
	}
}

func setStringSliceVariable(variables map[string]any, key string, values []string) {
	if len(values) > 0 {
		variables[key] = append([]string(nil), values...)
	}
}

func setPositiveIntVariable(variables map[string]any, key string, value int) {
	if value > 0 {
		variables[key] = value
	}
}

func setOptionalFloat(variables map[string]any, key string, value *float64) {
	if value != nil {
		variables[key] = *value
	}
}

func setOptionalBool(variables map[string]any, key string, value *bool) {
	if value != nil {
		variables[key] = *value
	}
}

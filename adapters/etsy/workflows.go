package etsy

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetShop(ctx context.Context, options ...socialhub.CallOption) (Shop, error) {
	const operation = "get_shop"
	var output Shop
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodGet, "/v3/application/shops/"+formatID(client.shopID), nil, nil, &output,
		http.StatusOK, false, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if output.ShopID != client.shopID {
		return output, platformContractError(operation, "Etsy returned a shop that did not match the configured shop")
	}
	return output, nil
}

func (client *Client) GetListing(
	ctx context.Context,
	listingID int64,
	input GetListingRequest,
	options ...socialhub.CallOption,
) (Listing, error) {
	const operation = "get_listing"
	if listingID <= 0 || !validGetListing(input) {
		return Listing{}, invalidArgument(operation, "listing ID, includes, or language is invalid")
	}
	if input.AllowSuggestedTitle != nil && *input.AllowSuggestedTitle && !client.hasOAuth {
		return Listing{}, approvalRequired(operation, "")
	}
	query := make(url.Values)
	addIncludes(query, input.Includes)
	setOptionalQuery(query, "language", input.Language)
	setOptionalBool(query, "allow_suggested_title", input.AllowSuggestedTitle)
	var output Listing
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodGet, "/v3/application/listings/"+formatID(listingID), query, nil, &output,
		http.StatusOK, false, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if output.ListingID != listingID {
		return output, platformContractError(operation, "Etsy returned a listing that did not match the requested listing")
	}
	return output, nil
}

func (client *Client) ListShopListings(
	ctx context.Context,
	input ListShopListingsRequest,
	options ...socialhub.CallOption,
) (ListingsResponse, error) {
	const operation = "list_shop_listings"
	if !client.hasOAuth {
		return ListingsResponse{}, approvalRequired(operation, "listings_r")
	}
	if !validListShopListings(input) {
		return ListingsResponse{}, invalidArgument(operation, "state, pagination, sort, or includes is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "state", string(input.State))
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	setOptionalQuery(query, "sort_on", string(input.SortOn))
	setOptionalQuery(query, "sort_order", string(input.SortOrder))
	addIncludes(query, input.Includes)
	var output ListingsResponse
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodGet, "/v3/application/shops/"+formatID(client.shopID)+"/listings", query, nil, &output,
		http.StatusOK, false, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if !validListingsResponse(output, client.shopID, input.Offset, input.Limit) {
		return output, platformContractError(operation, "Etsy returned an invalid or mismatched shop listing collection")
	}
	return output, nil
}

func (client *Client) CreateDraftListing(
	ctx context.Context,
	input CreateDraftListingRequest,
	options ...socialhub.CallOption,
) (Listing, error) {
	const operation = "create_draft_listing"
	if !client.hasOAuth {
		return Listing{}, approvalRequired(operation, "listings_w")
	}
	if !validCreateDraftListing(input) {
		return Listing{}, invalidArgument(operation, "required listing fields, exact price, enum, optional measurement, or ID is invalid")
	}
	form := url.Values{
		"quantity": {strconv.FormatInt(input.Quantity, 10)}, "title": {input.Title},
		"description": {input.Description}, "price": {string(input.Price)},
		"who_made": {string(input.WhoMade)}, "when_made": {string(input.WhenMade)},
		"taxonomy_id": {formatID(input.TaxonomyID)},
	}
	setOptionalInt64(form, "shipping_profile_id", input.ShippingProfileID)
	setOptionalInt64(form, "return_policy_id", input.ReturnPolicyID)
	addStrings(form, "materials", input.Materials)
	setOptionalInt64(form, "shop_section_id", input.ShopSectionID)
	setOptionalInt64(form, "processing_min", input.ProcessingMin)
	setOptionalInt64(form, "processing_max", input.ProcessingMax)
	setOptionalInt64(form, "readiness_state_id", input.ReadinessStateID)
	addStrings(form, "tags", input.Tags)
	addStrings(form, "styles", input.Styles)
	setOptionalFloat64(form, "item_weight", input.ItemWeight)
	setOptionalFloat64(form, "item_length", input.ItemLength)
	setOptionalFloat64(form, "item_width", input.ItemWidth)
	setOptionalFloat64(form, "item_height", input.ItemHeight)
	setOptionalQuery(form, "item_weight_unit", string(input.ItemWeightUnit))
	setOptionalQuery(form, "item_dimensions_unit", string(input.ItemDimensionsUnit))
	addInt64s(form, "production_partner_ids", input.ProductionPartnerIDs)
	addInt64s(form, "image_ids", input.ImageIDs)
	setOptionalBool(form, "is_supply", input.IsSupply)
	setOptionalBool(form, "is_customizable", input.IsCustomizable)
	setOptionalBool(form, "should_auto_renew", input.ShouldAutoRenew)
	setOptionalBool(form, "is_taxable", input.IsTaxable)
	setOptionalQuery(form, "type", string(input.Type))
	var output Listing
	metadata, raw, err := client.doForm(
		ctx, operation, "/v3/application/shops/"+formatID(client.shopID)+"/listings", form, &output,
		http.StatusCreated, true, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if output.ListingID <= 0 {
		return output, outcomeUnknownError(operation, unexpectedEmptyID(operation, "listing"), metadata.RequestID)
	}
	if output.ShopID != client.shopID {
		failure := platformContractError(operation, "Etsy createDraftListing returned a listing for another shop")
		return output, outcomeUnknownError(operation, failure, metadata.RequestID)
	}
	if output.State != ListingDraft {
		failure := platformContractError(operation, "Etsy createDraftListing returned a non-draft listing")
		return output, outcomeUnknownError(operation, failure, metadata.RequestID)
	}
	return output, nil
}

func (client *Client) ListListingImages(
	ctx context.Context,
	listingID int64,
	options ...socialhub.CallOption,
) (ListingImagesResponse, error) {
	const operation = "list_listing_images"
	if listingID <= 0 {
		return ListingImagesResponse{}, invalidArgument(operation, "listing ID must be positive")
	}
	var output ListingImagesResponse
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodGet, "/v3/application/listings/"+formatID(listingID)+"/images", nil, nil, &output,
		http.StatusOK, false, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if !validListingImagesResponse(output, listingID) {
		return output, platformContractError(operation, "Etsy returned an invalid or mismatched listing image collection")
	}
	return output, nil
}

func (client *Client) UploadListingImage(
	ctx context.Context,
	listingID int64,
	input UploadListingImageRequest,
	options ...socialhub.CallOption,
) (ListingImage, error) {
	const operation = "upload_listing_image"
	if !client.hasOAuth {
		return ListingImage{}, approvalRequired(operation, "listings_w")
	}
	if listingID <= 0 || !validUploadListingImage(input) {
		return ListingImage{}, invalidArgument(operation, "listing ID and exactly one image reader or existing listing image ID are required")
	}
	form := make(url.Values)
	if input.ListingImageID > 0 {
		form.Set("listing_image_id", formatID(input.ListingImageID))
	}
	setOptionalInt64(form, "rank", input.Rank)
	setOptionalBool(form, "overwrite", input.Overwrite)
	setOptionalBool(form, "is_watermarked", input.IsWatermarked)
	if input.AltText != nil {
		form.Set("alt_text", *input.AltText)
	}
	var output ListingImage
	metadata, raw, err := client.doMultipart(
		ctx, operation, "/v3/application/shops/"+formatID(client.shopID)+"/listings/"+formatID(listingID)+"/images",
		input, form, &output, http.StatusCreated, true, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if output.ListingImageID <= 0 {
		return output, outcomeUnknownError(operation, unexpectedEmptyID(operation, "listing image"), metadata.RequestID)
	}
	if output.ListingID != listingID {
		failure := platformContractError(operation, "Etsy uploadListingImage returned an image for another listing")
		return output, outcomeUnknownError(operation, failure, metadata.RequestID)
	}
	return output, nil
}

func (client *Client) GetListingInventory(
	ctx context.Context,
	listingID int64,
	input GetListingInventoryRequest,
	options ...socialhub.CallOption,
) (ListingInventory, error) {
	const operation = "get_listing_inventory"
	if !client.hasOAuth {
		return ListingInventory{}, approvalRequired(operation, "listings_r")
	}
	if listingID <= 0 {
		return ListingInventory{}, invalidArgument(operation, "listing ID must be positive")
	}
	query := make(url.Values)
	setOptionalBool(query, "show_deleted", input.ShowDeleted)
	if input.IncludeListing {
		query.Set("includes", "Listing")
	}
	var output ListingInventory
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodGet, "/v3/application/listings/"+formatID(listingID)+"/inventory", query, nil, &output,
		http.StatusOK, false, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if !validListingInventoryResponse(output, listingID, input.IncludeListing) {
		return output, platformContractError(operation, "Etsy returned an invalid or mismatched listing inventory")
	}
	return output, nil
}

func (client *Client) UpdateListingInventory(
	ctx context.Context,
	listingID int64,
	input UpdateListingInventoryRequest,
	options ...socialhub.CallOption,
) (ListingInventory, error) {
	const operation = "update_listing_inventory"
	if !client.hasOAuth {
		return ListingInventory{}, approvalRequired(operation, "listings_w")
	}
	if listingID <= 0 || !validUpdateInventory(input) {
		return ListingInventory{}, invalidArgument(operation, "listing ID, complete products, offerings, exact prices, property IDs, or max variations is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "max_variations_supported", input.MaxVariationsSupported)
	payload := normalizedInventoryRequest(input)
	var output ListingInventory
	metadata, raw, err := client.doJSON(
		ctx, operation, http.MethodPut, "/v3/application/listings/"+formatID(listingID)+"/inventory", query, payload, &output,
		http.StatusOK, true, options...,
	)
	output.Meta, output.Raw = metadata, raw
	if err != nil {
		return output, err
	}
	if !validListingInventoryResponse(output, listingID, false) {
		failure := platformContractError(operation, "Etsy returned invalid listing inventory after an update")
		return output, outcomeUnknownError(operation, failure, metadata.RequestID)
	}
	return output, nil
}

func normalizedInventoryRequest(input UpdateListingInventoryRequest) UpdateListingInventoryRequest {
	input.Products = append([]InventoryProductInput(nil), input.Products...)
	for index := range input.Products {
		if len(input.Products[index].PropertyValues) == 0 {
			input.Products[index].PropertyValues = []InventoryPropertyInput{}
		} else {
			input.Products[index].PropertyValues = append([]InventoryPropertyInput(nil), input.Products[index].PropertyValues...)
		}
		for propertyIndex := range input.Products[index].PropertyValues {
			property := &input.Products[index].PropertyValues[propertyIndex]
			if property.ValueIDs == nil {
				property.ValueIDs = []int64{}
			}
			if property.Values == nil {
				property.Values = []string{}
			}
		}
	}
	if input.PriceOnProperty == nil {
		input.PriceOnProperty = []int64{}
	}
	if input.QuantityOnProperty == nil {
		input.QuantityOnProperty = []int64{}
	}
	if input.SKUOnProperty == nil {
		input.SKUOnProperty = []int64{}
	}
	if input.ReadinessStateOnProperty == nil {
		input.ReadinessStateOnProperty = []int64{}
	}
	return input
}

func addIncludes(query url.Values, values []ListingInclude) {
	for _, value := range values {
		query.Add("includes", string(value))
	}
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setOptionalBool(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func setOptionalInt64(query url.Values, key string, value *int64) {
	if value != nil {
		query.Set(key, formatID(*value))
	}
}

func setOptionalFloat64(query url.Values, key string, value *float64) {
	if value != nil {
		query.Set(key, strconv.FormatFloat(*value, 'f', -1, 64))
	}
}

func addStrings(query url.Values, key string, values []string) {
	for _, value := range values {
		query.Add(key, value)
	}
}

func addInt64s(query url.Values, key string, values []int64) {
	for _, value := range values {
		query.Add(key, formatID(value))
	}
}

var _ ListingWorkflow = (*Client)(nil)

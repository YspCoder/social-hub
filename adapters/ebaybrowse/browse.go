package ebaybrowse

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) SearchItems(
	ctx context.Context,
	input SearchItemsRequest,
	options ...socialhub.CallOption,
) (SearchPage, error) {
	const operation = "search_items"
	if !validSearch(input) || !validRequestContext(input.Context, client.affiliateCampaignID) {
		return SearchPage{}, invalidArgument(operation, "search terms, filters, pagination, field groups, sort, or request context are invalid")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	query := make(url.Values)
	setOptional(query, "q", input.Query)
	setOptional(query, "gtin", input.GTIN)
	setOptional(query, "epid", input.EPID)
	setOptional(query, "category_ids", input.CategoryID)
	setOptional(query, "filter", input.Filter)
	setOptional(query, "aspect_filter", input.AspectFilter)
	setOptional(query, "compatibility_filter", input.CompatibilityFilter)
	setOptional(query, "fieldgroups", joinSearchFieldGroups(input.FieldGroups))
	setOptional(query, "sort", string(input.Sort))
	query.Set("limit", strconv.Itoa(limit))
	if input.Offset > 0 {
		query.Set("offset", strconv.Itoa(input.Offset))
	}
	var output SearchPage
	meta, err := client.getJSON(ctx, operation, "/item_summary/search", query, input.Context, &output, options...)
	if err != nil {
		return SearchPage{}, err
	}
	output.Meta = meta
	if err := validateSearchResponse(operation, output, limit, input.Offset); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) GetItem(
	ctx context.Context,
	input GetItemRequest,
	options ...socialhub.CallOption,
) (Item, error) {
	const operation = "get_item"
	if !validPathSegment(input.ItemID, 512) || !validItemFieldGroups(input.FieldGroups, true, true) ||
		!validQuantity(input.QuantityForShippingEstimate) || !validRequestContext(input.Context, client.affiliateCampaignID) {
		return Item{}, invalidArgument(operation, "item ID, field groups, shipping quantity, or request context are invalid")
	}
	query := itemQuery(input.FieldGroups, input.QuantityForShippingEstimate)
	var output Item
	meta, err := client.getJSON(ctx, operation, "/item/"+input.ItemID, query, input.Context, &output, options...)
	if err != nil {
		return Item{}, err
	}
	output.Meta = meta
	if err := validateItemResponse(operation, output, input.ItemID, ""); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) GetItemByLegacyID(
	ctx context.Context,
	input GetItemByLegacyIDRequest,
	options ...socialhub.CallOption,
) (Item, error) {
	const operation = "get_item_by_legacy_id"
	if !validOpaque(input.LegacyItemID, 256) || !validOptionalOpaque(input.LegacyVariationID, 256) ||
		!validOptionalOpaque(input.LegacyVariationSKU, 256) ||
		(input.LegacyVariationID != "" && input.LegacyVariationSKU != "") ||
		!validItemFieldGroups(input.FieldGroups, false, true) || !validQuantity(input.QuantityForShippingEstimate) ||
		!validRequestContext(input.Context, client.affiliateCampaignID) {
		return Item{}, invalidArgument(operation, "legacy IDs, field groups, shipping quantity, or request context are invalid")
	}
	query := itemQuery(input.FieldGroups, input.QuantityForShippingEstimate)
	query.Set("legacy_item_id", input.LegacyItemID)
	setOptional(query, "legacy_variation_id", input.LegacyVariationID)
	setOptional(query, "legacy_variation_sku", input.LegacyVariationSKU)
	var output Item
	meta, err := client.getJSON(ctx, operation, "/item/get_item_by_legacy_id", query, input.Context, &output, options...)
	if err != nil {
		return Item{}, err
	}
	output.Meta = meta
	if err := validateItemResponse(operation, output, "", input.LegacyItemID); err != nil {
		return output, err
	}
	return output, nil
}

func (client *Client) GetItemsByGroup(
	ctx context.Context,
	input GetItemsByGroupRequest,
	options ...socialhub.CallOption,
) (ItemGroup, error) {
	const operation = "get_items_by_group"
	if !validOpaque(input.ItemGroupID, 512) || !validItemFieldGroups(input.FieldGroups, false, false) ||
		!validQuantity(input.QuantityForShippingEstimate) || !validRequestContext(input.Context, client.affiliateCampaignID) {
		return ItemGroup{}, invalidArgument(operation, "item group ID, field groups, shipping quantity, or request context are invalid")
	}
	query := itemQuery(input.FieldGroups, input.QuantityForShippingEstimate)
	query.Set("item_group_id", input.ItemGroupID)
	var output ItemGroup
	meta, err := client.getJSON(ctx, operation, "/item/get_items_by_item_group", query, input.Context, &output, options...)
	if err != nil {
		return ItemGroup{}, err
	}
	output.Meta = meta
	if err := validateItemGroupResponse(operation, output); err != nil {
		return output, err
	}
	return output, nil
}

func itemQuery(fieldGroups []ItemFieldGroup, quantity int) url.Values {
	query := make(url.Values)
	if len(fieldGroups) > 0 {
		values := make([]string, len(fieldGroups))
		for index, value := range fieldGroups {
			values[index] = string(value)
		}
		query.Set("fieldgroups", strings.Join(values, ","))
	}
	if quantity > 0 {
		query.Set("quantity_for_shipping_estimate", strconv.Itoa(quantity))
	}
	return query
}

func joinSearchFieldGroups(values []SearchFieldGroup) string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = string(value)
	}
	return strings.Join(encoded, ",")
}

func setOptional(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

var _ BrowseWorkflow = (*Client)(nil)

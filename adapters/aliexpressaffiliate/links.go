package aliexpressaffiliate

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const linkGenerateMethod = "aliexpress.affiliate.link.generate"

func (client *Client) GenerateLinks(
	ctx context.Context,
	input LinkGenerationRequest,
	options ...socialhub.CallOption,
) (LinkGenerationResult, error) {
	const operation = "generate_links"
	trackingID, err := client.trackingID(operation, input.TrackingID, true)
	if err != nil {
		return LinkGenerationResult{}, err
	}
	if !validLinkGeneration(input) {
		return LinkGenerationResult{}, invalidArgument(operation, "link type, source values, ship-to country, or app signature are invalid")
	}
	values := make(url.Values)
	setString(values, "ship_to_country", input.ShipToCountry)
	setString(values, "app_signature", client.appSignature(input.AppSignature))
	values.Set("promotion_link_type", strconv.FormatInt(int64(input.PromotionLinkType), 10))
	values.Set("source_values", strings.Join(input.SourceValues, ","))
	values.Set("tracking_id", trackingID)
	var response struct {
		PromotionLinks   []PromotionLink `json:"promotion_links"`
		TotalResultCount ExactValue      `json:"total_result_count"`
		TrackingID       string          `json:"tracking_id"`
	}
	meta, err := client.doForm(ctx, operation, linkGenerateMethod, values, &response, options...)
	if err != nil {
		return LinkGenerationResult{}, err
	}
	return LinkGenerationResult{
		Links: response.PromotionLinks, TotalResultCount: response.TotalResultCount,
		TrackingID: response.TrackingID, Meta: meta,
	}, nil
}

func validLinkGeneration(input LinkGenerationRequest) bool {
	if input.PromotionLinkType != PromotionLinkStandard && input.PromotionLinkType != PromotionLinkHot ||
		len(input.SourceValues) == 0 || len(input.SourceValues) > 50 || !validCountry(input.ShipToCountry) ||
		input.AppSignature != "" && !validOpaque(input.AppSignature, 4096) ||
		input.TrackingID != "" && !validCSVValue(input.TrackingID, 512) {
		return false
	}
	for _, value := range input.SourceValues {
		if !validCSVValue(value, 8192) {
			return false
		}
	}
	return true
}

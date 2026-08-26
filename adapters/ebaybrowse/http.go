package ebaybrowse

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	requestContext RequestContext,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	client.applyContextHeaders(request, requestContext)
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, withOperation(err, operation)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return ResponseMeta{}, platformContractError(operation, "eBay returned an empty or invalid successful response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return ResponseMeta{}, platformContractError(operation, "eBay returned a non-JSON successful response")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return ResponseMeta{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		firstHeader(metadata.Header, "X-EBAY-C-REQUEST-ID", "X-EBAY-CORRELATION-ID", "X-Request-ID"),
		callOptions.RequestID,
	), 256)}, nil
}

func (client *Client) applyContextHeaders(request *http.Request, value RequestContext) {
	marketplaceID := value.MarketplaceID
	if marketplaceID == "" {
		marketplaceID = client.marketplaceID
	}
	request.Header.Set("X-EBAY-C-MARKETPLACE-ID", marketplaceID)
	language := value.AcceptLanguage
	if language == "" {
		language = client.acceptLanguage
	}
	if language != "" {
		request.Header.Set("Accept-Language", language)
	}
	parts := make([]string, 0, 3)
	if client.affiliateCampaignID != "" {
		parts = append(parts, "affiliateCampaignId="+encodeHeaderValue(client.affiliateCampaignID))
	}
	if value.AffiliateReferenceID != "" {
		parts = append(parts, "affiliateReferenceId="+encodeHeaderValue(value.AffiliateReferenceID))
	}
	if value.DeliveryCountry != "" {
		location := "country=" + value.DeliveryCountry + ",zip=" + value.DeliveryPostalCode
		parts = append(parts, "contextualLocation="+encodeHeaderValue(location))
	}
	if len(parts) > 0 {
		request.Header.Set("X-EBAY-C-ENDUSERCTX", strings.Join(parts, ","))
	}
}

func encodeHeaderValue(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	mediaType = strings.ToLower(mediaType)
	return err == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
}

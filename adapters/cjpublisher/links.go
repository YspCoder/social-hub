package cjpublisher

import (
	"context"
	"encoding/xml"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxLinkSearchResponseBytes = int64(8 << 20)

type Link struct {
	AdvertiserID         string
	AdvertiserName       string
	Category             string
	ClickCommission      string
	CreativeHeight       string
	CreativeWidth        string
	Language             string
	LeadCommission       string
	LinkCodeHTML         string
	LinkCodeJavaScript   string
	Destination          string
	LinkID               string
	LinkName             string
	Description          string
	LinkType             string
	AllowDeepLinking     bool
	PerformanceIncentive bool
	PromotionEndDate     string
	PromotionStartDate   string
	PromotionType        string
	CouponCode           string
	RelationshipStatus   string
	SaleCommission       string
	MobileOptimized      bool
	MobileAppDownload    bool
	CrossDeviceOnly      bool
	TargetedCountries    string
	EventName            string
	AdContent            string
	LastUpdated          string
	SevenDayEPC          string
	ThreeMonthEPC        string
	ClickURL             string
}

type LinksResponse struct {
	Links           []Link
	TotalMatched    int
	RecordsReturned int
	PageNumber      int
	Meta            ResponseMeta
	Raw             []byte
}

type linkSearchEnvelope struct {
	XMLName xml.Name       `xml:"cj-api"`
	Links   linkCollection `xml:"links"`
}

type linkCollection struct {
	TotalMatched    string     `xml:"total-matched,attr"`
	RecordsReturned string     `xml:"records-returned,attr"`
	PageNumber      string     `xml:"page-number,attr"`
	Links           []linkWire `xml:"link"`
}

type innerXML struct {
	Value string `xml:",innerxml"`
}

type linkWire struct {
	AdvertiserID         string     `xml:"advertiser-id"`
	AdvertiserName       string     `xml:"advertiser-name"`
	Category             string     `xml:"category"`
	ClickCommission      string     `xml:"click-commission"`
	CreativeHeight       string     `xml:"creative-height"`
	CreativeWidth        string     `xml:"creative-width"`
	Language             string     `xml:"language"`
	LeadCommission       string     `xml:"lead-commission"`
	LinkCodeHTML         innerXML   `xml:"link-code-html"`
	LinkCodeJavaScript   innerXML   `xml:"link-code-javascript"`
	Destination          string     `xml:"destination"`
	LinkID               string     `xml:"link-id"`
	LinkName             string     `xml:"link-name"`
	Description          string     `xml:"description"`
	LinkType             string     `xml:"link-type"`
	AllowDeepLinking     string     `xml:"allow-deep-linking"`
	PerformanceIncentive string     `xml:"performance-incentive"`
	PromotionEndDate     string     `xml:"promotion-end-date"`
	PromotionStartDate   string     `xml:"promotion-start-date"`
	PromotionType        string     `xml:"promotion-type"`
	CouponCode           string     `xml:"coupon-code"`
	RelationshipStatus   string     `xml:"relationship-status"`
	SaleCommission       string     `xml:"sale-commission"`
	MobileOptimized      string     `xml:"mobile-optimized"`
	MobileAppDownload    string     `xml:"mobile-app-download"`
	CrossDeviceOnly      string     `xml:"cross-device-only"`
	TargetedCountries    string     `xml:"targeted-countries"`
	EventName            string     `xml:"event-name"`
	AdContent            innerXML   `xml:"ad-content"`
	LastUpdated          string     `xml:"last-updated"`
	SevenDayEPC          string     `xml:"seven-day-epc"`
	ThreeMonthEPC        string     `xml:"three-month-epc"`
	ClickURL             string     `xml:"clickURL"`
	ClickURLAlt          string     `xml:"clickUrl"`
	Nested               []linkWire `xml:"link"`
}

func (client *Client) SearchLinks(
	ctx context.Context,
	input SearchLinksRequest,
	options ...socialhub.CallOption,
) (LinksResponse, error) {
	const operation = "search_links"
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return LinksResponse{}, err
	}
	websiteID := resolvePropertyID(input.WebsiteID, client.websiteID)
	if !validSearchLinks(input, websiteID) {
		return LinksResponse{}, invalidArgument(operation, "PID, required search filter, date dependency, paging, or enum is invalid")
	}
	query := make(url.Values)
	query.Set("website-id", websiteID)
	if len(input.AdvertiserIDs) > 0 {
		query.Set("advertiser-ids", strings.Join(input.AdvertiserIDs, ","))
	} else if input.Relationship != "" {
		query.Set("advertiser-ids", string(input.Relationship))
	}
	setQuery(query, "keywords", input.Keywords)
	if len(input.Categories) > 0 {
		query.Set("category", strings.Join(input.Categories, ","))
	}
	setQuery(query, "link-type", input.LinkType)
	setQuery(query, "promotion-type", input.PromotionType)
	setDateQuery(query, "promotion-start-date", input.PromotionStartDate)
	if input.OngoingPromotion {
		query.Set("promotion-end-date", "ongoing")
	} else {
		setDateQuery(query, "promotion-end-date", input.PromotionEndDate)
	}
	if input.PageNumber > 0 {
		query.Set("page-number", strconv.Itoa(input.PageNumber))
	}
	if input.RecordsPerPage > 0 {
		query.Set("records-per-page", strconv.Itoa(input.RecordsPerPage))
	}
	setQuery(query, "language", input.Language)
	setBoolQuery(query, "allow-deep-linking", input.AllowDeepLinking)
	setQuery(query, "event-name", input.EventName)
	setQuery(query, "link-id", input.LinkID)
	setDateQuery(query, "last-updated", input.LastUpdated)
	setBoolQuery(query, "cross-device-only", input.CrossDeviceOnly)
	setBoolQuery(query, "mobile-app-download", input.MobileAppDownload)
	setBoolQuery(query, "mobile-optimized", input.MobileOptimized)
	setQuery(query, "targeted-country", input.TargetedCountry)

	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.linksAPI.NewRequest(ctx, http.MethodGet, "/v2/link-search", query, nil)
	if err != nil {
		return LinksResponse{}, withOperation(err, operation)
	}
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return LinksResponse{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxLinkSearchResponseBytes+1))
	meta := ResponseMeta{RequestID: boundedMessage(firstNonEmpty(
		firstHeader(response.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
	), 256)}
	if err != nil {
		return LinksResponse{Meta: meta}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(raw)) > maxLinkSearchResponseBytes {
		return LinksResponse{Meta: meta}, platformContractError(operation, "CJ Link Search response exceeds 8 MiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return LinksResponse{Meta: meta}, withOperation(client.errorDecoder(response.StatusCode, response.Header, raw), operation)
	}
	if response.StatusCode != http.StatusOK {
		return LinksResponse{Meta: meta}, platformContractError(operation, "CJ returned an unexpected successful Link Search status")
	}
	if !validXMLContentType(response.Header.Get("Content-Type")) {
		return LinksResponse{Meta: meta}, platformContractError(operation, "CJ returned a non-XML Link Search response")
	}
	var envelope linkSearchEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return LinksResponse{Meta: meta, Raw: append([]byte(nil), raw...)}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if envelope.XMLName.Local != "cj-api" {
		return LinksResponse{Meta: meta, Raw: append([]byte(nil), raw...)}, platformContractError(operation, "CJ Link Search response has an unexpected XML root")
	}
	result := LinksResponse{Meta: meta, Raw: append([]byte(nil), raw...)}
	if result.TotalMatched, err = parseNonnegativeInt(envelope.Links.TotalMatched); err != nil {
		return result, platformContractError(operation, "CJ returned invalid total-matched metadata")
	}
	if result.RecordsReturned, err = parseNonnegativeInt(envelope.Links.RecordsReturned); err != nil {
		return result, platformContractError(operation, "CJ returned invalid records-returned metadata")
	}
	if result.PageNumber, err = parseNonnegativeInt(envelope.Links.PageNumber); err != nil {
		return result, platformContractError(operation, "CJ returned invalid page-number metadata")
	}
	links, err := flattenLinks(envelope.Links.Links, 0)
	if err != nil {
		return result, platformContractError(operation, err.Error())
	}
	result.Links = links
	if err := validateLinksResponse(operation, result, input); err != nil {
		return result, err
	}
	return result, nil
}

func flattenLinks(values []linkWire, depth int) ([]Link, error) {
	if depth > 8 {
		return nil, &linkDecodeError{message: "CJ Link Search nesting exceeds the supported depth"}
	}
	result := make([]Link, 0, len(values))
	for _, value := range values {
		if len(value.Nested) > 0 {
			nested, err := flattenLinks(value.Nested, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, nested...)
			continue
		}
		allowDeepLinking, err := parseProviderBool(value.AllowDeepLinking)
		if err != nil {
			return nil, err
		}
		performanceIncentive, err := parseProviderBool(value.PerformanceIncentive)
		if err != nil {
			return nil, err
		}
		mobileOptimized, err := parseProviderBool(value.MobileOptimized)
		if err != nil {
			return nil, err
		}
		mobileAppDownload, err := parseProviderBool(value.MobileAppDownload)
		if err != nil {
			return nil, err
		}
		crossDeviceOnly, err := parseProviderBool(value.CrossDeviceOnly)
		if err != nil {
			return nil, err
		}
		result = append(result, Link{
			AdvertiserID: value.AdvertiserID, AdvertiserName: value.AdvertiserName,
			Category: value.Category, ClickCommission: value.ClickCommission,
			CreativeHeight: value.CreativeHeight, CreativeWidth: value.CreativeWidth,
			Language: value.Language, LeadCommission: value.LeadCommission,
			LinkCodeHTML: value.LinkCodeHTML.Value, LinkCodeJavaScript: value.LinkCodeJavaScript.Value,
			Destination: value.Destination, LinkID: value.LinkID, LinkName: value.LinkName,
			Description: value.Description, LinkType: value.LinkType,
			AllowDeepLinking: allowDeepLinking, PerformanceIncentive: performanceIncentive,
			PromotionEndDate: value.PromotionEndDate, PromotionStartDate: value.PromotionStartDate,
			PromotionType: value.PromotionType, CouponCode: value.CouponCode,
			RelationshipStatus: value.RelationshipStatus, SaleCommission: value.SaleCommission,
			MobileOptimized: mobileOptimized, MobileAppDownload: mobileAppDownload,
			CrossDeviceOnly: crossDeviceOnly, TargetedCountries: value.TargetedCountries,
			EventName: value.EventName, AdContent: value.AdContent.Value, LastUpdated: value.LastUpdated,
			SevenDayEPC: value.SevenDayEPC, ThreeMonthEPC: value.ThreeMonthEPC,
			ClickURL: firstNonEmpty(value.ClickURL, value.ClickURLAlt),
		})
	}
	return result, nil
}

type linkDecodeError struct{ message string }

func (value *linkDecodeError) Error() string { return value.message }

func parseProviderBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true, nil
	case "", "false", "no", "0":
		return false, nil
	default:
		return false, &linkDecodeError{message: "CJ Link Search returned an invalid boolean"}
	}
}

func parseNonnegativeInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, &linkDecodeError{message: "invalid nonnegative integer"}
	}
	return parsed, nil
}

func validXMLContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "application/xml", "text/xml", "application/octet-stream":
		return true
	default:
		return false
	}
}

func setQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setDateQuery(query url.Values, key string, value time.Time) {
	if !value.IsZero() {
		query.Set(key, value.Format("01/02/2006"))
	}
}

func setBoolQuery(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

package merchantapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validNumericID(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	nonZero := false
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		nonZero = nonZero || value[index] != '0'
	}
	_, err := strconv.ParseUint(value, 10, 63)
	return nonZero && err == nil
}

func validOptionalUint(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 20 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 63)
	return err == nil
}

func validOpaque(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalText(value string, maximum int) bool {
	return value == "" || len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validPageToken(value string) bool {
	return value == "" || validOpaque(value, 16384)
}

func validListRequest(input ListRequest, maximum int) bool {
	return input.PageSize >= 0 && input.PageSize <= maximum && validPageToken(input.PageToken)
}

func effectivePageSize(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func validLanguageCode(value string) bool {
	if value == "" {
		return true
	}
	if len(value) < 2 || len(value) > 35 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validTimeZone(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 128 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("/_+-", character) {
			continue
		}
		return false
	}
	return true
}

func validIssueListRequest(input IssueListRequest) bool {
	return input.PageSize >= 0 && input.PageSize <= 100 && validPageToken(input.PageToken) &&
		validLanguageCode(input.LanguageCode) && validTimeZone(input.TimeZone)
}

func validAccountName(accountID, value string) bool {
	return value == "accounts/"+accountID
}

func validChildResourceName(accountID, collection, value string) bool {
	prefix := "accounts/" + accountID + "/" + collection + "/"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	return validResourceSegment(strings.TrimPrefix(value, prefix))
}

func validOpaqueChildResourceName(accountID, collection, value string) bool {
	prefix := "accounts/" + accountID + "/" + collection + "/"
	return strings.HasPrefix(value, prefix) && validOpaque(strings.TrimPrefix(value, prefix), 2048)
}

func validResourceSegment(value string) bool {
	if value == "" || len(value) > 2048 {
		return false
	}
	for index := range value {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-._~", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func validDataSourceName(accountID, value string) bool {
	prefix := "accounts/" + accountID + "/dataSources/"
	return strings.HasPrefix(value, prefix) && validNumericID(strings.TrimPrefix(value, prefix), 20)
}

func validDataSource(accountID string, value DataSource) bool {
	if !validDataSourceName(accountID, value.Name) || !validOptionalText(value.DisplayName, 4096) {
		return false
	}
	return value.DataSourceID == "" || validNumericID(value.DataSourceID, 20) &&
		strings.HasSuffix(value.Name, "/"+value.DataSourceID)
}

func validProductName(accountID, value string) bool {
	return validChildResourceName(accountID, "products", value)
}

func validProductInputName(accountID, value string) bool {
	return validChildResourceName(accountID, "productInputs", value)
}

func validReturnedProductName(accountID, collection, value string) bool {
	_, valid := productResourceIdentifier(accountID, collection, value)
	return valid
}

func productResourceIdentifier(accountID, collection, value string) (string, bool) {
	prefix := "accounts/" + accountID + "/" + collection + "/"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	identifier := strings.TrimPrefix(value, prefix)
	if identifier == "" || len(identifier) > 4096 || !utf8.ValidString(identifier) {
		return "", false
	}
	if !strings.Contains(identifier, "~") {
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(identifier)
		if err != nil {
			return "", false
		}
		identifier = string(decoded)
	}
	return identifier, validProductIdentifier(identifier)
}

func validProductIdentifier(value string) bool {
	parts := strings.SplitN(value, "~", 3)
	if strings.HasPrefix(value, "local~") {
		parts = strings.SplitN(value, "~", 4)
		return len(parts) == 4 && parts[0] == "local" && validContentLanguage(parts[1]) &&
			validFeedLabel(parts[2]) && validOfferID(parts[3])
	}
	return len(parts) == 3 && validContentLanguage(parts[0]) && validFeedLabel(parts[1]) && validOfferID(parts[2])
}

func sameProductResource(accountID, collection, left, right string) bool {
	leftID, leftValid := productResourceIdentifier(accountID, collection, left)
	rightID, rightValid := productResourceIdentifier(accountID, collection, right)
	return leftValid && rightValid && leftID == rightID
}

func matchesProductResourceName(accountID, collection, requested, name, encodedName string) bool {
	return sameProductResource(accountID, collection, requested, name) ||
		encodedName != "" && sameProductResource(accountID, collection, requested, encodedName)
}

func validOfferID(value string) bool {
	return value != "" && utf8.ValidString(value) && strings.TrimSpace(value) == value &&
		utf8.RuneCountInString(value) <= 50 && !strings.ContainsFunc(value, unicode.IsControl)
}

func validContentLanguage(value string) bool {
	return len(value) == 2 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z'
}

func validFeedLabel(value string) bool {
	if value == "" || len(value) > 20 {
		return false
	}
	for index := range value {
		character := value[index]
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

// EncodeProductID returns Google's recommended unpadded base64url identifier
// for a v1 Product or ProductInput resource.
func EncodeProductID(contentLanguage, feedLabel, offerID string, legacyLocal bool) (string, error) {
	if !validContentLanguage(contentLanguage) || !validFeedLabel(feedLabel) || !validOfferID(offerID) {
		return "", fmt.Errorf("merchantapi: content language, feed label, or offer ID is invalid")
	}
	offerID = strings.Join(strings.Fields(offerID), " ")
	parts := []string{contentLanguage, feedLabel, offerID}
	if legacyLocal {
		parts = append([]string{"local"}, parts...)
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "~"))), nil
}

func ProductResourceName(accountID, contentLanguage, feedLabel, offerID string, legacyLocal bool) (string, error) {
	if !validNumericID(accountID, 20) {
		return "", fmt.Errorf("merchantapi: account ID is invalid")
	}
	identifier, err := EncodeProductID(contentLanguage, feedLabel, offerID, legacyLocal)
	if err != nil {
		return "", err
	}
	return "accounts/" + accountID + "/products/" + identifier, nil
}

func ProductInputResourceName(accountID, contentLanguage, feedLabel, offerID string, legacyLocal bool) (string, error) {
	if !validNumericID(accountID, 20) {
		return "", fmt.Errorf("merchantapi: account ID is invalid")
	}
	identifier, err := EncodeProductID(contentLanguage, feedLabel, offerID, legacyLocal)
	if err != nil {
		return "", err
	}
	return "accounts/" + accountID + "/productInputs/" + identifier, nil
}

func validPrice(value *Price) bool {
	if value == nil {
		return true
	}
	if value.AmountMicros == "" || !validOptionalUint(value.AmountMicros) || len(value.CurrencyCode) != 3 {
		return false
	}
	for index := range value.CurrencyCode {
		if value.CurrencyCode[index] < 'A' || value.CurrencyCode[index] > 'Z' {
			return false
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func validOptionalHTTPURL(value string) bool {
	return value == "" || len(value) <= 4096 && validHTTPURL(value)
}

func validAvailability(value Availability) bool {
	switch value {
	case "", AvailabilityUnspecified, AvailabilityInStock, AvailabilityOutOfStock, AvailabilityPreorder, AvailabilityLimited, AvailabilityBackorder:
		return true
	default:
		return false
	}
}

func validCondition(value Condition) bool {
	switch value {
	case "", ConditionUnspecified, ConditionNew, ConditionRefurbished, ConditionUsed:
		return true
	default:
		return false
	}
}

func validPause(value Pause) bool {
	return value == "" || value == PauseUnspecified || value == PauseAds || value == PauseAll
}

func validProductDestination(value ProductDestination) bool {
	switch value {
	case DestinationUnspecified, DestinationShoppingAds, DestinationDisplayAds, DestinationLocalInventoryAds,
		DestinationFreeListings, DestinationFreeLocalListings, DestinationYouTubeShopping, DestinationYouTubeCheckout,
		DestinationYouTubeAffiliate, DestinationFreeVehicleListings, DestinationVehicleAds, DestinationCloudRetail,
		DestinationLocalCloudRetail:
		return true
	default:
		return false
	}
}

func validGTIN(value string) bool {
	if len(value) != 8 && len(value) != 12 && len(value) != 13 && len(value) != 14 {
		return false
	}
	for index := range value {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func validProductAttributes(value *ProductAttributes) bool {
	if value == nil {
		return true
	}
	const maximumItems = 4096
	if len(value.AdditionalImageLinks) > maximumItems || len(value.VideoLinks) > maximumItems ||
		len(value.ProductTypes) > maximumItems || len(value.IncludedDestinations) > maximumItems ||
		len(value.ExcludedDestinations) > maximumItems || len(value.Extra) > maximumItems {
		return false
	}
	if !validOptionalText(value.Title, 600) || !validOptionalText(value.Description, 20000) ||
		!validOptionalHTTPURL(value.Link) || !validOptionalHTTPURL(value.CanonicalLink) || !validOptionalHTTPURL(value.ImageLink) ||
		!validAvailability(value.Availability) || !validCondition(value.Condition) || !validPrice(value.Price) ||
		!validPrice(value.SalePrice) || !validPause(value.Pause) || len(value.GTINs) > 10 ||
		!validOptionalText(value.AvailabilityDate, 256) || !validOptionalText(value.Brand, 4096) ||
		!validOptionalText(value.MPN, 4096) || !validOptionalText(value.GoogleProductCategory, 4096) ||
		!validOptionalText(value.ItemGroupID, 4096) || !validOptionalText(value.Color, 4096) ||
		!validOptionalText(value.Size, 4096) || !validOptionalText(value.ExpirationDate, 256) {
		return false
	}
	total := len(value.Title) + len(value.Description) + len(value.Link) + len(value.CanonicalLink) + len(value.ImageLink) +
		len(value.AvailabilityDate) + len(value.Brand) + len(value.MPN) + len(value.GoogleProductCategory) +
		len(value.ItemGroupID) + len(value.Color) + len(value.Size) + len(value.ExpirationDate)
	for _, candidate := range value.AdditionalImageLinks {
		total += len(candidate)
		if total > 1<<20 || !validOptionalHTTPURL(candidate) || candidate == "" {
			return false
		}
	}
	for _, candidate := range value.VideoLinks {
		total += len(candidate)
		if total > 1<<20 || !validOptionalHTTPURL(candidate) || candidate == "" {
			return false
		}
	}
	for _, gtin := range value.GTINs {
		if !validGTIN(gtin) {
			return false
		}
		total += len(gtin)
	}
	for _, productType := range value.ProductTypes {
		total += len(productType)
		if total > 1<<20 || !validOptionalText(productType, 4096) || productType == "" {
			return false
		}
	}
	for _, label := range []string{value.CustomLabel0, value.CustomLabel1, value.CustomLabel2, value.CustomLabel3, value.CustomLabel4} {
		if !validOptionalText(label, 400) {
			return false
		}
		total += len(label)
	}
	for _, destinations := range [][]ProductDestination{value.IncludedDestinations, value.ExcludedDestinations} {
		for _, destination := range destinations {
			total += len(destination)
			if total > 1<<20 || !validProductDestination(destination) {
				return false
			}
		}
	}
	for name, raw := range value.Extra {
		total += len(name) + len(raw)
		if total > 1<<20 || knownProductAttributeField(name) || !validJSONFieldName(name) || !validJSONValue(raw) {
			return false
		}
	}
	if total > 1<<20 {
		return false
	}
	encoded, err := json.Marshal(value)
	return err == nil && len(encoded) <= 1<<20 && validJSONObjectValue(encoded)
}

func validCustomAttributes(values []CustomAttribute) bool {
	if len(values) > 2500 {
		return false
	}
	total := 0
	for _, value := range values {
		if !validCustomAttribute(value, 0, &total) {
			return false
		}
	}
	return total <= 102400
}

func validCustomAttribute(value CustomAttribute, depth int, total *int) bool {
	if depth > 16 || !validOpaque(value.Name, 10240) || !utf8.ValidString(value.Value) || strings.ContainsRune(value.Value, '\x00') {
		return false
	}
	if value.Value != "" && len(value.GroupValues) > 0 || value.Value == "" && len(value.GroupValues) == 0 {
		return false
	}
	*total += utf8.RuneCountInString(value.Name) + utf8.RuneCountInString(value.Value)
	if *total > 102400 || utf8.RuneCountInString(value.Name)+utf8.RuneCountInString(value.Value) > 10240 {
		return false
	}
	for _, child := range value.GroupValues {
		if !validCustomAttribute(child, depth+1, total) {
			return false
		}
	}
	return true
}

func validInsertProductInput(accountID string, input InsertProductInputRequest) bool {
	value := input.Input
	return validDataSourceName(accountID, input.DataSource) && value.Name == "" && value.Product == "" &&
		value.Base64EncodedName == "" && value.Base64EncodedProduct == "" && validOfferID(value.OfferID) &&
		validContentLanguage(value.ContentLanguage) && validFeedLabel(value.FeedLabel) &&
		validOptionalUint(value.VersionNumber) && validProductAttributes(value.ProductAttributes) &&
		validCustomAttributes(value.CustomAttributes)
}

func validUpdateMask(values []string) bool {
	if len(values) == 0 || len(values) > 256 {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if len(value) > 256 || !validProductUpdateMaskPath(value) {
			return false
		}
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validProductUpdateMaskPath(value string) bool {
	prefix, field, found := strings.Cut(value, ".")
	if !found || field == "" || strings.Contains(field, ".") ||
		prefix != "product_attributes" && prefix != "custom_attribute" {
		return false
	}
	return validFieldMaskSegment(field)
}

func validFieldMaskSegment(value string) bool {
	if value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validPatchProductInput(accountID string, input PatchProductInputRequest) bool {
	value := input.Input
	return validDataSourceName(accountID, input.DataSource) && validProductInputName(accountID, input.Name) &&
		(value.Name == "" || value.Name == input.Name) && value.Product == "" && value.Base64EncodedName == "" &&
		value.Base64EncodedProduct == "" && value.OfferID == "" && value.ContentLanguage == "" &&
		value.FeedLabel == "" && !value.LegacyLocal && value.VersionNumber == "" && validUpdateMask(input.UpdateMask) &&
		validProductAttributes(value.ProductAttributes) && validCustomAttributes(value.CustomAttributes)
}

func validDeleteProductInput(accountID string, input DeleteProductInputRequest) bool {
	return validDataSourceName(accountID, input.DataSource) && validProductInputName(accountID, input.Name)
}

func validProductInput(accountID string, value ProductInput) bool {
	return validReturnedProductName(accountID, "productInputs", value.Name) &&
		(value.Product == "" || validReturnedProductName(accountID, "products", value.Product)) &&
		(value.Base64EncodedName == "" || validProductInputName(accountID, value.Base64EncodedName)) &&
		(value.Base64EncodedProduct == "" || validProductName(accountID, value.Base64EncodedProduct)) &&
		(value.Base64EncodedName == "" || sameProductResource(accountID, "productInputs", value.Name, value.Base64EncodedName)) &&
		(value.Product == "" || value.Base64EncodedProduct == "" || sameProductResource(accountID, "products", value.Product, value.Base64EncodedProduct)) &&
		validOfferID(value.OfferID) && validContentLanguage(value.ContentLanguage) && validFeedLabel(value.FeedLabel) &&
		productIdentityMatches(accountID, "productInputs", value.Name, value.Base64EncodedName, value.ContentLanguage, value.FeedLabel, value.OfferID, value.LegacyLocal) &&
		validOptionalUint(value.VersionNumber) && validProductAttributes(value.ProductAttributes) && validCustomAttributes(value.CustomAttributes)
}

func validProduct(accountID string, value Product) bool {
	return validReturnedProductName(accountID, "products", value.Name) &&
		(value.Base64EncodedName == "" || validProductName(accountID, value.Base64EncodedName)) &&
		(value.Base64EncodedName == "" || sameProductResource(accountID, "products", value.Name, value.Base64EncodedName)) &&
		(value.DataSource == "" || validDataSourceName(accountID, value.DataSource)) &&
		validOfferID(value.OfferID) && validContentLanguage(value.ContentLanguage) && validFeedLabel(value.FeedLabel) &&
		productIdentityMatches(accountID, "products", value.Name, value.Base64EncodedName, value.ContentLanguage, value.FeedLabel, value.OfferID, value.LegacyLocal) &&
		validOptionalUint(value.VersionNumber) && validProductAttributes(value.ProductAttributes) && validCustomAttributes(value.CustomAttributes)
}

func productIdentityMatches(accountID, collection, name, encodedName, language, feedLabel, offerID string, legacyLocal bool) bool {
	parts := []string{language, feedLabel, strings.Join(strings.Fields(offerID), " ")}
	if legacyLocal {
		parts = append([]string{"local"}, parts...)
	}
	expected := strings.Join(parts, "~")
	identifier, valid := productResourceIdentifier(accountID, collection, name)
	if !valid || identifier != expected {
		return false
	}
	if encodedName == "" {
		return true
	}
	identifier, valid = productResourceIdentifier(accountID, collection, encodedName)
	return valid && identifier == expected
}

func validReportRequest(input ReportRequest) bool {
	return validQuery(input.Query) && input.PageSize >= 0 && input.PageSize <= 100000 && validPageToken(input.PageToken)
}

func validQuery(value string) bool {
	if !validOpaque(value, 65536) || strings.Contains(value, ";") || strings.Contains(value, "--") ||
		strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return false
	}
	upper := strings.ToUpper(strings.Join(strings.Fields(value), " "))
	return strings.HasPrefix(upper, "SELECT ") && strings.Contains(upper, " FROM ")
}

func validJSONFieldName(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

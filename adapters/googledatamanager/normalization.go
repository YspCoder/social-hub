package googledatamanager

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

func normalizeRequest(input IngestEventsRequest) (IngestEventsRequest, error) {
	if len(input.Destinations) == 0 || len(input.Destinations) > MaximumDestinationsPerRequest {
		return IngestEventsRequest{}, fmt.Errorf("destinations must contain between 1 and %d entries", MaximumDestinationsPerRequest)
	}
	if len(input.Events) == 0 || len(input.Events) > MaximumEventsPerRequest {
		return IngestEventsRequest{}, fmt.Errorf("events must contain between 1 and %d entries", MaximumEventsPerRequest)
	}
	if !validConsent(input.Consent) {
		return IngestEventsRequest{}, fmt.Errorf("consent is invalid")
	}
	references := make(map[string]struct{}, len(input.Destinations))
	destinations := append([]Destination(nil), input.Destinations...)
	for index := range destinations {
		if err := validateDestination(destinations[index]); err != nil {
			return IngestEventsRequest{}, fmt.Errorf("destinations[%d]: %w", index, err)
		}
		if destinations[index].Reference != "" {
			if _, found := references[destinations[index].Reference]; found {
				return IngestEventsRequest{}, fmt.Errorf("destinations[%d].reference must be unique", index)
			}
			references[destinations[index].Reference] = struct{}{}
		}
	}
	hasUserData := false
	events := make([]Event, len(input.Events))
	for index := range input.Events {
		if input.Events[index].UserData != nil || input.Events[index].ThirdPartyUserData != nil {
			hasUserData = true
		}
	}
	if hasUserData {
		if !validEncoding(input.Encoding) {
			return IngestEventsRequest{}, fmt.Errorf("encoding must be HEX or BASE64 when user data is present")
		}
		if input.EncryptionInfo != nil && !validEncryptionInfo(input.EncryptionInfo) {
			return IngestEventsRequest{}, fmt.Errorf("encryption_info must contain exactly one valid GCP or AWS wrapped key")
		}
	} else if input.Encoding != "" || input.EncryptionInfo != nil {
		return IngestEventsRequest{}, fmt.Errorf("encoding and encryption_info require user data")
	}
	for index := range input.Events {
		event, err := normalizeEvent(input.Events[index], input.Encoding, input.EncryptionInfo != nil, references)
		if err != nil {
			return IngestEventsRequest{}, fmt.Errorf("events[%d]: %w", index, err)
		}
		events[index] = event
	}
	output := input
	output.Destinations = destinations
	output.Events = events
	return output, nil
}

func validateDestination(value Destination) error {
	if !validOptionalOpaque(value.Reference, 256) || !validProductAccount(value.OperatingAccount) ||
		!validOpaque(value.ProductDestinationID, 1024) {
		return fmt.Errorf("reference, operating_account, or product_destination_id is invalid")
	}
	if value.LoginAccount != nil && !validProductAccount(*value.LoginAccount) {
		return fmt.Errorf("login_account is invalid")
	}
	if value.LinkedAccount != nil && !validProductAccount(*value.LinkedAccount) {
		return fmt.Errorf("linked_account is invalid")
	}
	if value.OperatingAccount.AccountType == AccountTypeGoogleAds && !validNumericID(value.ProductDestinationID) {
		return fmt.Errorf("Google Ads product_destination_id must be a nonzero numeric conversion action ID")
	}
	return nil
}

func validProductAccount(value ProductAccount) bool {
	return validAccountType(value.AccountType) && validOpaque(value.AccountID, 256)
}

func normalizeEvent(input Event, encoding Encoding, encrypted bool, references map[string]struct{}) (Event, error) {
	if input.EventTimestamp.IsZero() {
		return Event{}, fmt.Errorf("event_timestamp is required")
	}
	if !validEventSource(input.EventSource) || !validConsent(input.Consent) || !validCurrency(input.Currency) {
		return Event{}, fmt.Errorf("event_source, consent, or currency is invalid")
	}
	for _, value := range []string{input.TransactionID, input.ClientID, input.AppInstanceID, input.UserID, input.EventName} {
		if !validText(value, 16*1024, false) {
			return Event{}, fmt.Errorf("event contains an invalid text field")
		}
	}
	if input.ConversionValue != "" && !validDecimal(input.ConversionValue) ||
		input.ConversionCount != "" && !validNonNegativeDecimal(input.ConversionCount) {
		return Event{}, fmt.Errorf("conversion_value or conversion_count is invalid")
	}
	if err := validateReferences(input.DestinationReferences, references, "destination_references"); err != nil {
		return Event{}, err
	}
	effectiveReferences := references
	if len(input.DestinationReferences) > 0 {
		effectiveReferences = make(map[string]struct{}, len(input.DestinationReferences))
		for _, reference := range input.DestinationReferences {
			effectiveReferences[reference] = struct{}{}
		}
	}
	userData, err := normalizeUserData(input.UserData, encoding, encrypted)
	if err != nil {
		return Event{}, fmt.Errorf("user_data: %w", err)
	}
	thirdParty, err := normalizeUserData(input.ThirdPartyUserData, encoding, encrypted)
	if err != nil {
		return Event{}, fmt.Errorf("third_party_user_data: %w", err)
	}
	if err := validateAdIdentifiers(input.AdIdentifiers); err != nil {
		return Event{}, err
	}
	if err := validateDeviceInfo(input.EventDeviceInfo); err != nil {
		return Event{}, fmt.Errorf("event_device_info: %w", err)
	}
	if err := validateEventLocation(input.EventLocation); err != nil {
		return Event{}, err
	}
	if err := validateCartData(input.CartData, effectiveReferences); err != nil {
		return Event{}, err
	}
	if err := validateCustomVariables(input.CustomVariables, effectiveReferences); err != nil {
		return Event{}, err
	}
	if err := validateExperimentalFields(input.ExperimentalFields); err != nil {
		return Event{}, err
	}
	if err := validateUserProperties(input.UserProperties); err != nil {
		return Event{}, err
	}
	if err := validateEventParameters(input.AdditionalEventParameters, "additional_event_parameters"); err != nil {
		return Event{}, err
	}
	output := input
	output.DestinationReferences = append([]string(nil), input.DestinationReferences...)
	output.UserData = userData
	output.ThirdPartyUserData = thirdParty
	return output, nil
}

func normalizeUserData(input *UserData, encoding Encoding, encrypted bool) (*UserData, error) {
	if input == nil {
		return nil, nil
	}
	if len(input.UserIdentifiers) == 0 || len(input.UserIdentifiers) > MaximumUserIdentifiers {
		return nil, fmt.Errorf("user_identifiers must contain between 1 and %d entries", MaximumUserIdentifiers)
	}
	output := &UserData{UserIdentifiers: make([]UserIdentifier, len(input.UserIdentifiers))}
	for index, identifier := range input.UserIdentifiers {
		normalized, err := normalizeUserIdentifier(identifier, encoding, encrypted)
		if err != nil {
			return nil, fmt.Errorf("user_identifiers[%d]: %w", index, err)
		}
		output.UserIdentifiers[index] = normalized
	}
	return output, nil
}

func normalizeUserIdentifier(input UserIdentifier, encoding Encoding, encrypted bool) (UserIdentifier, error) {
	if boolInt(input.EmailAddress != "")+boolInt(input.PhoneNumber != "")+boolInt(input.Address != nil) != 1 {
		return UserIdentifier{}, fmt.Errorf("exactly one identifier must be set")
	}
	if encrypted {
		if input.EmailAddress != "" && !validEncoded(input.EmailAddress, encoding, false) ||
			input.PhoneNumber != "" && !validEncoded(input.PhoneNumber, encoding, false) {
			return UserIdentifier{}, fmt.Errorf("encrypted identifier is not valid for the selected encoding")
		}
		if input.Address != nil {
			address, err := normalizeEncryptedAddress(*input.Address, encoding)
			if err != nil {
				return UserIdentifier{}, err
			}
			return UserIdentifier{Address: &address}, nil
		}
		return input, nil
	}
	if input.EmailAddress != "" {
		value, err := normalizeEmail(input.EmailAddress, encoding)
		return UserIdentifier{EmailAddress: value}, err
	}
	if input.PhoneNumber != "" {
		value, err := normalizePhone(input.PhoneNumber, encoding)
		return UserIdentifier{PhoneNumber: value}, err
	}
	address, err := normalizeAddress(*input.Address, encoding)
	return UserIdentifier{Address: &address}, err
}

func normalizeEmail(value string, encoding Encoding) (string, error) {
	if validEncoded(value, encoding, true) {
		return canonicalEncoded(value, encoding), nil
	}
	value = strings.ToLower(removeWhitespace(value))
	separator := strings.LastIndexByte(value, '@')
	if separator <= 0 || separator == len(value)-1 || strings.ContainsRune(value[separator+1:], '@') {
		return "", fmt.Errorf("email_address is invalid")
	}
	local, domain := value[:separator], value[separator+1:]
	if domain == "gmail.com" || domain == "googlemail.com" {
		if plus := strings.IndexByte(local, '+'); plus >= 0 {
			local = local[:plus]
		}
		local = strings.ReplaceAll(local, ".", "")
	}
	normalized := local + "@" + domain
	if !emailPattern.MatchString(normalized) {
		return "", fmt.Errorf("email_address is invalid")
	}
	return hashAndEncode(normalized, encoding), nil
}

func normalizePhone(value string, encoding Encoding) (string, error) {
	if validEncoded(value, encoding, true) {
		return canonicalEncoded(value, encoding), nil
	}
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 16 || value[0] != '+' {
		return "", fmt.Errorf("phone_number must use E.164 format")
	}
	for _, character := range value[1:] {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("phone_number must use E.164 format")
		}
	}
	return hashAndEncode(value, encoding), nil
}

func normalizeAddress(input AddressInfo, encoding Encoding) (AddressInfo, error) {
	given, err := normalizeName(input.GivenName, encoding)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("given_name is invalid")
	}
	family, err := normalizeName(input.FamilyName, encoding)
	if err != nil {
		return AddressInfo{}, fmt.Errorf("family_name is invalid")
	}
	region := strings.ToUpper(strings.TrimSpace(input.RegionCode))
	postal := strings.TrimSpace(input.PostalCode)
	if !validRegionCode(region) || !validOpaque(postal, 64) {
		return AddressInfo{}, fmt.Errorf("region_code or postal_code is invalid")
	}
	line := ""
	if input.AddressLine != "" {
		if validEncoded(input.AddressLine, encoding, true) {
			line = canonicalEncoded(input.AddressLine, encoding)
		} else {
			normalized := normalizeAddressText(input.AddressLine)
			if normalized == "" {
				return AddressInfo{}, fmt.Errorf("address_line is invalid")
			}
			line = hashAndEncode(normalized, encoding)
		}
	}
	city := normalizeAddressText(input.City)
	area := normalizeAddressText(input.AdministrativeArea)
	if input.City != "" && city == "" || input.AdministrativeArea != "" && area == "" {
		return AddressInfo{}, fmt.Errorf("city or administrative_area is invalid")
	}
	return AddressInfo{
		GivenName: given, FamilyName: family, RegionCode: region, PostalCode: postal,
		AddressLine: line, City: city, AdministrativeArea: area,
	}, nil
}

func normalizeEncryptedAddress(input AddressInfo, encoding Encoding) (AddressInfo, error) {
	if !validEncoded(input.GivenName, encoding, false) || !validEncoded(input.FamilyName, encoding, false) ||
		input.AddressLine != "" && !validEncoded(input.AddressLine, encoding, false) {
		return AddressInfo{}, fmt.Errorf("encrypted address hash field is not valid for the selected encoding")
	}
	region := strings.ToUpper(strings.TrimSpace(input.RegionCode))
	postal := strings.TrimSpace(input.PostalCode)
	if !validRegionCode(region) || !validOpaque(postal, 64) {
		return AddressInfo{}, fmt.Errorf("region_code or postal_code is invalid")
	}
	city := normalizeAddressText(input.City)
	area := normalizeAddressText(input.AdministrativeArea)
	if input.City != "" && city == "" || input.AdministrativeArea != "" && area == "" {
		return AddressInfo{}, fmt.Errorf("city or administrative_area is invalid")
	}
	return AddressInfo{
		GivenName: input.GivenName, FamilyName: input.FamilyName, RegionCode: region, PostalCode: postal,
		AddressLine: input.AddressLine, City: city, AdministrativeArea: area,
	}, nil
}

func normalizeName(value string, encoding Encoding) (string, error) {
	if validEncoded(value, encoding, true) {
		return canonicalEncoded(value, encoding), nil
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if !validText(value, 1024, true) {
		return "", fmt.Errorf("invalid name")
	}
	return hashAndEncode(value, encoding), nil
}

func normalizeAddressText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(character rune) rune {
		if unicode.IsPunct(character) || unicode.IsSymbol(character) || unicode.IsControl(character) {
			return -1
		}
		return character
	}, value)
	return strings.TrimSpace(value)
}

func removeWhitespace(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) {
			return -1
		}
		return character
	}, value)
}

func hashAndEncode(value string, encoding Encoding) string {
	digest := sha256.Sum256([]byte(value))
	if encoding == EncodingBase64 {
		return base64.StdEncoding.EncodeToString(digest[:])
	}
	return hex.EncodeToString(digest[:])
}

func canonicalEncoded(value string, encoding Encoding) string {
	if encoding == EncodingHex {
		return strings.ToLower(value)
	}
	return value
}

func validateReferences(values []string, references map[string]struct{}, field string) error {
	if len(values) > MaximumDestinationsPerRequest {
		return fmt.Errorf("%s contains too many entries", field)
	}
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		if !validOpaque(value, 256) {
			return fmt.Errorf("%s[%d] is invalid", field, index)
		}
		if _, found := references[value]; !found {
			return fmt.Errorf("%s[%d] does not match a destination reference", field, index)
		}
		if _, found := seen[value]; found {
			return fmt.Errorf("%s[%d] is duplicated", field, index)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateAdIdentifiers(value *AdIdentifiers) error {
	if value == nil {
		return nil
	}
	for _, candidate := range []string{
		value.SessionAttributes, value.GCLID, value.GBRAID, value.WBRAID, value.MobileDeviceID,
		value.DCLID, value.ImpressionID, value.MatchID,
	} {
		if !validOptionalOpaque(candidate, 16*1024) {
			return fmt.Errorf("ad_identifiers contains an invalid identifier")
		}
	}
	if err := validateDeviceInfo(value.LandingPageDeviceInfo); err != nil {
		return fmt.Errorf("ad_identifiers.landing_page_device_info: %w", err)
	}
	for index, identifier := range value.EncryptedUserIDs {
		if !validOpaque(identifier.EncryptedID, 16*1024) || !validEncryptionEntityType(identifier.EntityType) ||
			!validNumericID(identifier.EntityID) || !validEncryptionSource(identifier.Source) {
			return fmt.Errorf("ad_identifiers.encrypted_user_ids[%d] is invalid", index)
		}
	}
	return nil
}

func validateDeviceInfo(value *DeviceInfo) error {
	if value == nil {
		return nil
	}
	for _, candidate := range []string{
		value.UserAgent, value.Category, value.OperatingSystem, value.OperatingSystemVersion,
		value.Model, value.Brand, value.Browser, value.BrowserVersion,
	} {
		if !validText(candidate, 16*1024, false) {
			return fmt.Errorf("device contains an invalid text field")
		}
	}
	if !validIPAddress(value.IPAddress) || !validLanguageCode(value.LanguageCode) || value.ScreenHeight < 0 || value.ScreenWidth < 0 {
		return fmt.Errorf("IP address, language code, or screen dimensions are invalid")
	}
	return nil
}

func validateEventLocation(value *EventLocation) error {
	if value == nil {
		return nil
	}
	for _, candidate := range []string{
		value.StoreID, value.City, value.SubdivisionCode, value.RegionCode, value.SubcontinentCode, value.ContinentCode,
	} {
		if !validText(candidate, 4096, false) {
			return fmt.Errorf("event_location contains an invalid text field")
		}
	}
	return nil
}

func validateCartData(value *CartData, references map[string]struct{}) error {
	if value == nil {
		return nil
	}
	for _, candidate := range []string{value.MerchantID, value.MerchantFeedLabel, value.MerchantFeedLanguageCode} {
		if !validText(candidate, 4096, false) {
			return fmt.Errorf("cart_data contains an invalid merchant field")
		}
	}
	if value.TransactionDiscount != "" && !validNonNegativeDecimal(value.TransactionDiscount) {
		return fmt.Errorf("cart_data.transaction_discount is invalid")
	}
	for index, coupon := range value.CouponCodes {
		if !validText(coupon, 4096, true) {
			return fmt.Errorf("cart_data.coupon_codes[%d] is invalid", index)
		}
	}
	for index, item := range value.Items {
		if err := validateItem(item, references); err != nil {
			return fmt.Errorf("cart_data.items[%d]: %w", index, err)
		}
	}
	return nil
}

func validateItem(value Item, references map[string]struct{}) error {
	for _, candidate := range []string{
		value.MerchantProductID, value.ItemID, value.MerchantID, value.MerchantFeedLabel, value.MerchantFeedLanguageCode,
	} {
		if !validText(candidate, 4096, false) {
			return fmt.Errorf("item contains an invalid text field")
		}
	}
	if !validQuantity(value.Quantity) || value.UnitPrice != "" && !validNonNegativeDecimal(value.UnitPrice) ||
		value.ConversionValue != "" && !validDecimal(value.ConversionValue) {
		return fmt.Errorf("quantity, unit_price, or conversion_value is invalid")
	}
	if err := validateItemParameters(value.AdditionalItemParameters); err != nil {
		return err
	}
	for index, variable := range value.CustomVariables {
		if !validText(variable.Variable, 4096, false) || !validText(variable.Value, 4096, false) ||
			(variable.Variable == "" && variable.Value == "") {
			return fmt.Errorf("custom_variables[%d] is invalid", index)
		}
		if err := validateReferences(variable.DestinationReferences, references, fmt.Sprintf("custom_variables[%d].destination_references", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomVariables(values []CustomVariable, references map[string]struct{}) error {
	for index, value := range values {
		if !validText(value.Variable, 4096, false) || !validText(value.Value, 4096, false) ||
			(value.Variable == "" && value.Value == "") {
			return fmt.Errorf("custom_variables[%d] is invalid", index)
		}
		if err := validateReferences(value.DestinationReferences, references, fmt.Sprintf("custom_variables[%d].destination_references", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateExperimentalFields(values []ExperimentalField) error {
	for index, value := range values {
		if !validText(value.Field, 4096, false) || !validText(value.Value, 4096, false) ||
			(value.Field == "" && value.Value == "") {
			return fmt.Errorf("experimental_fields[%d] is invalid", index)
		}
	}
	return nil
}

func validateUserProperties(value *UserProperties) error {
	if value == nil {
		return nil
	}
	if !validCustomerType(value.CustomerType) || !validCustomerValueBucket(value.CustomerValueBucket) {
		return fmt.Errorf("user_properties customer type or value bucket is invalid")
	}
	for index, property := range value.AdditionalUserProperties {
		if !validText(property.PropertyName, 4096, true) || !validText(property.Value, 16*1024, true) {
			return fmt.Errorf("user_properties.additional_user_properties[%d] is invalid", index)
		}
	}
	return nil
}

func validateEventParameters(values []EventParameter, field string) error {
	for index, parameter := range values {
		if !validText(parameter.ParameterName, 4096, true) || !validText(parameter.Value, 16*1024, true) {
			return fmt.Errorf("%s[%d] is invalid", field, index)
		}
	}
	return nil
}

func validateItemParameters(values []ItemParameter) error {
	for index, parameter := range values {
		if !validText(parameter.ParameterName, 4096, true) || !validText(parameter.Value, 16*1024, true) {
			return fmt.Errorf("additional_item_parameters[%d] is invalid", index)
		}
	}
	return nil
}

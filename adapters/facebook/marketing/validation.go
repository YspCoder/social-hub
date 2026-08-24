package marketing

import (
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validNumericID(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validObjectID(value string) bool {
	if !validOpaque(value, 256) || strings.ContainsAny(value, "/?#") {
		return false
	}
	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' && character != '-' {
			return false
		}
	}
	return true
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
	return len(value) <= maximum && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validRequiredText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value && validOptionalText(value, maximum)
}

func validEnumToken(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '_' && character != '-' && !unicode.IsUpper(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validFieldName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '_' && !unicode.IsLower(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validPublicURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validMutationStatus(value Status) bool {
	return value == StatusActive || value == StatusPaused || value == StatusArchived || value == StatusDeleted
}

func validateBudget(daily, lifetime int64) bool {
	return daily >= 0 && lifetime >= 0 && (daily == 0 || lifetime == 0)
}

func validatePage(cursor string, maximum int) (int, error) {
	if maximum < 0 || maximum > 100 {
		return 0, invalidArgument("pagination", "max_results must be between 0 and 100")
	}
	if cursor != "" && !validOpaque(cursor, 4096) {
		return 0, invalidArgument("pagination", "cursor is invalid")
	}
	if maximum == 0 {
		maximum = 50
	}
	return maximum, nil
}

func validateStatuses(values []string) bool {
	for _, value := range values {
		if !validEnumToken(value) {
			return false
		}
	}
	return true
}

func validateTargeting(input TargetingSpec) bool {
	if input.AgeMin != 0 && (input.AgeMin < 13 || input.AgeMin > 65) {
		return false
	}
	if input.AgeMax != 0 && (input.AgeMax < 13 || input.AgeMax > 65 || input.AgeMin != 0 && input.AgeMax < input.AgeMin) {
		return false
	}
	for _, gender := range input.Genders {
		if gender != 1 && gender != 2 {
			return false
		}
	}
	if input.GeoLocations == nil || !validateGeoLocations(*input.GeoLocations) {
		return false
	}
	for _, list := range [][]TargetRef{input.CustomAudiences, input.ExcludedCustomAudiences} {
		if !validateTargetRefs(list) {
			return false
		}
	}
	for _, flexible := range input.FlexibleSpec {
		if !validateTargetRefs(flexible.Interests) || !validateTargetRefs(flexible.Behaviors) || !validateTargetRefs(flexible.LifeEvents) ||
			!validateTargetRefs(flexible.WorkEmployers) || !validateTargetRefs(flexible.WorkPositions) {
			return false
		}
	}
	for _, values := range [][]string{
		input.PublisherPlatforms, input.FacebookPositions, input.InstagramPositions,
		input.MessengerPositions, input.AudienceNetworkPositions, input.DevicePlatforms,
	} {
		for _, value := range values {
			if !validFieldName(value) {
				return false
			}
		}
	}
	return true
}

func validateGeoLocations(input GeoLocations) bool {
	if len(input.Countries)+len(input.Cities)+len(input.Regions)+len(input.Zips) == 0 {
		return false
	}
	for _, country := range input.Countries {
		if len(country) != 2 || country != strings.ToUpper(country) {
			return false
		}
	}
	for _, target := range append(append(append([]GeoTarget(nil), input.Cities...), input.Regions...), input.Zips...) {
		if !validObjectID(target.Key) || !validOptionalText(target.Name, 256) || target.Radius < 0 || target.Radius > 1000 {
			return false
		}
	}
	for _, locationType := range input.LocationTypes {
		if !validFieldName(locationType) {
			return false
		}
	}
	return true
}

func validateTargetRefs(values []TargetRef) bool {
	for _, value := range values {
		if !validNumericID(value.ID) || !validOptionalText(value.Name, 512) {
			return false
		}
	}
	return true
}

func validatePromotedObject(value *PromotedObject) bool {
	if value == nil {
		return true
	}
	if value.PageID == "" && value.PixelID == "" && value.CustomConversionID == "" {
		return false
	}
	for _, id := range []string{value.PageID, value.PixelID, value.CustomConversionID} {
		if id != "" && !validNumericID(id) {
			return false
		}
	}
	return value.CustomEventType == "" || validEnumToken(value.CustomEventType)
}

func validateObjectStorySpec(value *ObjectStorySpec) bool {
	if value == nil || !validNumericID(value.PageID) || value.LinkData == nil {
		return false
	}
	link := value.LinkData
	if !validPublicURL(link.Link) || !validOptionalText(link.Message, 20_000) || !validOptionalText(link.Name, 1024) ||
		!validOptionalText(link.Description, 4096) || link.ImageHash != "" && !validObjectID(link.ImageHash) {
		return false
	}
	if link.CallToAction != nil {
		if !validEnumToken(link.CallToAction.Type) || !validPublicURL(link.CallToAction.Value.Link) {
			return false
		}
	}
	return true
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

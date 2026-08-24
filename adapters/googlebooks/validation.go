package googlebooks

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxSecretReferenceLength = 4_096
	maxCredentialLength      = 16_384
	maxQueryLength           = 4_096
	maxVolumeIDLength        = 1_024
)

func validSensitive(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalSensitive(value string, maximum int) bool {
	return value == "" || validSensitive(value, maximum)
}

func validCredential(value string) bool {
	return validSensitive(value, maxCredentialLength) && !strings.ContainsFunc(value, unicode.IsSpace)
}

func onlyBooksScope(scopes []string) bool {
	return len(scopes) == 1 && scopes[0] == ScopeBooks
}

func prepareCallOptions(operation string, options []socialhub.CallOption) (socialhub.CallOptions, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return socialhub.CallOptions{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.IdempotencyKey != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "idempotency keys are not supported by public Volume reads")
	}
	if len(resolved.Fields) != 0 {
		return socialhub.CallOptions{}, invalidArgument(operation, "partial-response field masks are not exposed by this typed adapter")
	}
	if resolved.RequestID != "" {
		return socialhub.CallOptions{}, invalidArgument(operation, "Google Books API does not document a caller request-ID header")
	}
	return resolved, nil
}

func normalizeSearchRequest(input SearchVolumesRequest) (SearchVolumesRequest, error) {
	if !validSensitive(input.Query, maxQueryLength) {
		return SearchVolumesRequest{}, invalidArgument("search_volumes", "query is required and invalid")
	}
	if input.StartIndex < 0 {
		return SearchVolumesRequest{}, invalidArgument("search_volumes", "start index must not be negative")
	}
	if input.MaxResults == 0 {
		input.MaxResults = DefaultMaxResults
	}
	if input.MaxResults < 1 || input.MaxResults > MaximumMaxResults {
		return SearchVolumesRequest{}, invalidArgument("search_volumes", "max results must be between 1 and 40")
	}
	if !validVolumeFilter(input.Filter) || !validVolumeOrder(input.OrderBy) ||
		!validSearchPrintType(input.PrintType) || !validProjection(input.Projection) {
		return SearchVolumesRequest{}, invalidArgument("search_volumes", "search filter, order, print type, or projection is invalid")
	}
	if input.Language != "" {
		if len(input.Language) != 2 || !asciiLetter(input.Language[0]) || !asciiLetter(input.Language[1]) {
			return SearchVolumesRequest{}, invalidArgument("search_volumes", "language must be a two-letter ISO 639-1 code")
		}
		input.Language = strings.ToLower(input.Language)
	}
	return input, nil
}

func normalizeGetRequest(input GetVolumeRequest) (GetVolumeRequest, error) {
	if !validVolumeID(input.VolumeID) {
		return GetVolumeRequest{}, invalidArgument("get_volume", "volume ID is required and invalid")
	}
	if !validProjection(input.Projection) {
		return GetVolumeRequest{}, invalidArgument("get_volume", "projection is invalid")
	}
	return input, nil
}

func validVolumeFilter(value VolumeFilter) bool {
	return value == "" || value == VolumeFilterEBooks || value == VolumeFilterFreeEBooks ||
		value == VolumeFilterFull || value == VolumeFilterPaidEBooks ||
		value == VolumeFilterPartial
}

func validVolumeOrder(value VolumeOrder) bool {
	return value == "" || value == VolumeOrderRelevance || value == VolumeOrderNewest
}

func validSearchPrintType(value SearchPrintType) bool {
	return value == "" || value == SearchPrintAll || value == SearchPrintBooks || value == SearchPrintMagazines
}

func validProjection(value Projection) bool {
	return value == "" || value == ProjectionFull || value == ProjectionLite
}

func asciiLetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validVolumeID(value string) bool {
	if !validSensitive(value, maxVolumeIDLength) {
		return false
	}
	return !strings.ContainsAny(value, "/\\?#")
}

func validateVolumePage(response volumesResponse, request SearchVolumesRequest) error {
	if response.Kind != "books#volumes" || response.TotalItems < 0 || len(response.Items) > request.MaxResults {
		return platformContractError("search_volumes", "Google Books returned an invalid Volume page")
	}
	for _, volume := range response.Items {
		if err := validateVolume("search_volumes", volume, ""); err != nil {
			return err
		}
	}
	return nil
}

func validateVolume(operation string, volume Volume, expectedID string) error {
	if volume.Kind != "books#volume" || !validVolumeID(volume.ID) || expectedID != "" && volume.ID != expectedID {
		return platformContractError(operation, "Google Books returned an invalid or mismatched Volume resource")
	}
	return nil
}

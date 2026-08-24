package github

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.RawQuery == "" && parsed.Fragment == "" && !strings.HasSuffix(value, "/")
}

func validOpaque(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validOptionalOpaque(value string, maximum int) bool {
	return value == "" || validOpaque(value, maximum)
}

func validAccessToken(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func validStringSet(values []string, maximum int) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validOpaque(value, maximum) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validPathSegment(value string) bool {
	return validOpaque(value, 256) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validDecimalID(value string) bool {
	return validDecimalIDBytes([]byte(value))
}

func validDecimalIDBytes(value []byte) bool {
	if len(value) == 0 || len(value) > 19 || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(string(value), 10, 64)
	return err == nil
}

func validPagination(perPage, page int) bool {
	return perPage >= 0 && perPage <= MaximumPageSize && page >= 0
}

func validDirection(value Direction) bool {
	switch value {
	case "", DirectionAscending, DirectionDescending:
		return true
	default:
		return false
	}
}

func validRepositorySort(value RepositorySort) bool {
	switch value {
	case "", RepositorySortCreated, RepositorySortUpdated, RepositorySortPushed, RepositorySortFullName:
		return true
	default:
		return false
	}
}

func validAuthenticatedRepositoryRequest(input ListAuthenticatedRepositoriesRequest) bool {
	switch input.Visibility {
	case "", RepositoryVisibilityAll, RepositoryVisibilityPublic, RepositoryVisibilityPrivate:
	default:
		return false
	}
	switch input.Type {
	case "", RepositoryTypeAll, RepositoryTypeOwner, RepositoryTypePublic, RepositoryTypePrivate, RepositoryTypeMember:
	default:
		return false
	}
	seen := make(map[RepositoryAffiliation]struct{}, len(input.Affiliation))
	for _, value := range input.Affiliation {
		switch value {
		case RepositoryAffiliationOwner, RepositoryAffiliationCollaborator, RepositoryAffiliationOrganizationMember:
		default:
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	if input.Type != "" && (input.Visibility != "" || len(input.Affiliation) > 0) {
		return false
	}
	return validRepositorySort(input.Sort) && validDirection(input.Direction) && validPagination(input.PerPage, input.Page) &&
		(input.Since == nil || !input.Since.IsZero()) && (input.Before == nil || !input.Before.IsZero())
}

func validUserRepositoryRequest(input ListUserRepositoriesRequest) bool {
	switch input.Type {
	case "", RepositoryTypeAll, RepositoryTypeOwner, RepositoryTypeMember:
	default:
		return false
	}
	return validRepositorySort(input.Sort) && validDirection(input.Direction) && validPagination(input.PerPage, input.Page)
}

func validIssueRequest(input ListIssuesRequest) bool {
	switch input.State {
	case "", IssueStateOpen, IssueStateClosed, IssueStateAll:
	default:
		return false
	}
	switch input.Sort {
	case "", IssueSortCreated, IssueSortUpdated, IssueSortComments:
	default:
		return false
	}
	if !validDirection(input.Direction) || !validPagination(input.PerPage, input.Page) || input.Since != nil && input.Since.IsZero() {
		return false
	}
	seen := make(map[string]struct{}, len(input.Labels))
	for _, label := range input.Labels {
		if !validOpaque(label, 256) || strings.Contains(label, ",") {
			return false
		}
		if _, exists := seen[label]; exists {
			return false
		}
		seen[label] = struct{}{}
	}
	return true
}

func validIssueCommentRequest(input ListIssueCommentsRequest) bool {
	return validPagination(input.PerPage, input.Page) && (input.Since == nil || !input.Since.IsZero())
}

func validUser(value User) bool {
	return value.ID != "" && value.Login != "" && validOpaque(value.Login, 256)
}

func validRepository(value Repository) bool {
	return value.ID != "" && validOpaque(value.Name, 256) && validOpaque(value.FullName, 512) && value.Owner != nil && validUser(*value.Owner)
}

func validIssue(value Issue) bool {
	return value.ID != "" && value.Number != "" && value.URL != "" && value.RepositoryURL != ""
}

func validIssueComment(value IssueComment) bool {
	return value.ID != "" && value.URL != "" && value.IssueURL != ""
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "GitHub REST does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only GitHub REST operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed GitHub REST operation")
	}
	return nil
}

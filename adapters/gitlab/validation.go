package gitlab

import (
	"net/url"
	"strconv"
	"strings"
	"time"
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

func validSort(value SortDirection) bool {
	switch value {
	case "", SortAscending, SortDescending:
		return true
	default:
		return false
	}
}

func validListProjects(input ListProjectsRequest) bool {
	switch input.Visibility {
	case "", ProjectVisibilityPrivate, ProjectVisibilityInternal, ProjectVisibilityPublic:
	default:
		return false
	}
	switch input.OrderBy {
	case "", ProjectOrderID, ProjectOrderName, ProjectOrderPath, ProjectOrderCreatedAt,
		ProjectOrderUpdatedAt, ProjectOrderLastActivityAt, ProjectOrderStarCount:
	default:
		return false
	}
	return validSort(input.Sort) && validOptionalOpaque(input.Search, 512) && validPagination(input.PerPage, input.Page)
}

func validListProjectIssues(input ListProjectIssuesRequest) bool {
	switch input.State {
	case "", IssueStateOpened, IssueStateClosed, IssueStateAll:
	default:
		return false
	}
	switch input.OrderBy {
	case "", IssueOrderCreatedAt, IssueOrderUpdatedAt:
	default:
		return false
	}
	if !validSort(input.Sort) || !validOptionalOpaque(input.Search, 1024) || !validPagination(input.PerPage, input.Page) {
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
	for _, value := range []*time.Time{input.CreatedAfter, input.CreatedBefore, input.UpdatedAfter, input.UpdatedBefore} {
		if value != nil && value.IsZero() {
			return false
		}
	}
	return true
}

func validListIssueNotes(input ListIssueNotesRequest) bool {
	switch input.ActivityFilter {
	case "", NoteActivityAll, NoteActivityComments, NoteActivitySystem:
	default:
		return false
	}
	switch input.OrderBy {
	case "", NoteOrderCreatedAt, NoteOrderUpdatedAt:
	default:
		return false
	}
	return validSort(input.Sort) && validPagination(input.PerPage, input.Page)
}

func validUser(value User) bool {
	return value.ID != "" && validOpaque(value.Username, 256) && validOpaque(value.Name, 512)
}

func validProject(value Project) bool {
	return value.ID != "" && validOpaque(value.Name, 512) && validOpaque(value.Path, 512) &&
		validOpaque(value.PathWithNamespace, 1024) && value.WebURL != ""
}

func validIssue(value Issue) bool {
	return value.ID != "" && value.IID != "" && value.ProjectID != "" && validOpaque(value.Title, 1024) && value.WebURL != ""
}

func validNote(value Note) bool {
	return value.ID != "" && value.NoteableID != "" && value.NoteableIID != "" && value.ProjectID != "" && value.NoteableType == "Issue"
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "GitLab REST v4 does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only GitLab REST operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed GitLab REST operation")
	}
	return nil
}

package bitbucket

import (
	"net/mail"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

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

func validCredential(value string) bool {
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

func validEmail(value string) bool {
	if !validOpaque(value, 320) || strings.ContainsAny(value, "\r\n:") {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func validAccountSettings(settings AccountSettings) bool {
	switch settings.AuthMode {
	case AuthBearer:
		return settings.Email == ""
	case AuthBasicAPIToken:
		return validEmail(settings.Email)
	default:
		return false
	}
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

func containsScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if scope == target {
			return true
		}
	}
	return false
}

func validResourceSelector(value string) bool {
	return validOpaque(value, 256) && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#%")
}

func validID(value ID) bool { return value > 0 }

func validPageOptions(options PageOptions) bool {
	if options.NextQuery != "" {
		return options.PageLength == 0 && validContinuationQuery(options.NextQuery)
	}
	return options.PageLength == 0 || options.PageLength >= MinimumPageLength && options.PageLength <= MaximumPageLength
}

func validContinuationQuery(value string) bool {
	if !validOpaque(value, 16_384) || strings.HasPrefix(value, "?") || strings.Contains(value, "#") {
		return false
	}
	query, err := url.ParseQuery(value)
	if err != nil || len(query) == 0 {
		return false
	}
	for key := range query {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
		switch normalized {
		case "access_token", "api_token", "authorization", "client_secret", "password", "refresh_token":
			return false
		}
	}
	return true
}

func validWorkspaceRequest(input ListWorkspacesRequest) bool {
	if !validPageOptions(input.Page) {
		return false
	}
	if input.Page.NextQuery != "" {
		return input.Administrator == nil && input.Sort == ""
	}
	switch input.Sort {
	case "", WorkspaceSortSlugAscending, WorkspaceSortSlugDescending:
		return true
	default:
		return false
	}
}

func validRepositoryRequest(input ListRepositoriesRequest) bool {
	if !validPageOptions(input.Page) || !validOptionalOpaque(input.Query, 4096) || !validOptionalOpaque(input.Sort, 256) {
		return false
	}
	if input.Page.NextQuery != "" {
		return input.Role == "" && input.Query == "" && input.Sort == ""
	}
	switch input.Role {
	case "", RepositoryRoleAdmin, RepositoryRoleContributor, RepositoryRoleMember, RepositoryRoleOwner:
		return true
	default:
		return false
	}
}

func validPullRequestRequest(input ListPullRequestsRequest) bool {
	if !validPageOptions(input.Page) || !validOptionalOpaque(input.Query, 4096) || !validOptionalOpaque(input.Sort, 256) {
		return false
	}
	if input.Page.NextQuery != "" {
		return len(input.States) == 0 && input.Query == "" && input.Sort == ""
	}
	seen := make(map[PullRequestState]struct{}, len(input.States))
	for _, state := range input.States {
		switch state {
		case PullRequestStateOpen, PullRequestStateMerged, PullRequestStateDeclined, PullRequestStateSuperseded:
		default:
			return false
		}
		if _, exists := seen[state]; exists {
			return false
		}
		seen[state] = struct{}{}
	}
	return true
}

func validPullRequestCommentRequest(input ListPullRequestCommentsRequest) bool {
	if !validPageOptions(input.Page) || !validOptionalOpaque(input.Query, 4096) || !validOptionalOpaque(input.Sort, 256) {
		return false
	}
	return input.Page.NextQuery == "" || input.Query == "" && input.Sort == ""
}

func validAccount(value Account) bool {
	return validOpaque(value.UUID, 256)
}

func validWorkspaceBase(value WorkspaceBase) bool {
	return validOpaque(value.UUID, 256) && validOpaque(value.Slug, 256)
}

func validWorkspaceAccess(value WorkspaceAccess) bool {
	return validWorkspaceBase(value.Workspace)
}

func validWorkspace(value Workspace) bool {
	return validOpaque(value.UUID, 256) && validOpaque(value.Slug, 256)
}

func validRepository(value Repository) bool {
	return validOpaque(value.UUID, 256) && validOpaque(value.FullName, 512) && validOpaque(value.Name, 256)
}

func validPullRequest(value PullRequest) bool {
	return validID(value.ID)
}

func validPullRequestComment(value PullRequestComment) bool {
	return validID(value.ID)
}

func matchesWorkspaceSelector(value *WorkspaceBase, selector string) bool {
	return value != nil && validWorkspaceBase(*value) && (value.Slug == selector || value.UUID == selector)
}

func matchesWorkspaceDetail(value Workspace, selector string) bool {
	return validWorkspace(value) && (value.Slug == selector || value.UUID == selector)
}

func matchesRepositorySelector(value *Repository, workspace, repository string) bool {
	return value != nil && validRepository(*value) && matchesWorkspaceSelector(value.Workspace, workspace) &&
		(value.Slug == repository || value.UUID == repository)
}

func matchesPullRequest(value PullRequest, workspace, repository string, id ID) bool {
	return validPullRequest(value) && value.ID == id && matchesRepositorySelector(value.Destination.Repository, workspace, repository)
}

func matchesPullRequestComment(value PullRequestComment, pullRequestID ID) bool {
	return validPullRequestComment(value) && value.PullRequest != nil && value.PullRequest.ID == pullRequestID
}

func prepareCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if resolved.RequestID != "" {
		return invalidArgument(operation, "Bitbucket Cloud REST does not document a caller request-ID header")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "read-only Bitbucket operations do not use idempotency keys")
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed Bitbucket operation")
	}
	return nil
}

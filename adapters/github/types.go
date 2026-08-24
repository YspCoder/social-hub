package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumPageSize        = 100
	maxProviderObjectBytes = 8 << 20
)

// ID preserves GitHub's int64 JSON identifiers as decimal strings so callers
// never lose precision when values cross language boundaries.
type ID string

func (value *ID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if !validDecimalIDBytes(trimmed) {
		return fmt.Errorf("github: ID must be a positive int64 JSON number")
	}
	*value = ID(string(trimmed))
	return nil
}

// ResponseMeta preserves GitHub's request, version, cache, scope, SSO, rate,
// and lifecycle headers. Plan- and authentication-dependent values remain text.
type ResponseMeta struct {
	RequestID           string
	APIVersionSelected  string
	MediaType           string
	RateLimitLimit      string
	RateLimitRemaining  string
	RateLimitUsed       string
	RateLimitReset      string
	RateLimitResource   string
	RetryAfter          string
	OAuthScopes         []string
	AcceptedOAuthScopes []string
	SSO                 string
	ETag                string
	Link                string
	Deprecation         string
	Sunset              string
	Warning             string
	RateLimitResetAt    *time.Time
	RateLimitResetAfter time.Duration
	RetryAfterDuration  time.Duration
}

// PageLinks contains page numbers parsed from Link. Absolute provider URLs are
// retained only in Raw and are never accepted as subsequent request targets.
type PageLinks struct {
	Raw          string
	NextPage     *int
	PreviousPage *int
	FirstPage    *int
	LastPage     *int
}

// Page is one provider page with the exact response metadata and bounded JSON.
type Page[T any] struct {
	Items []T
	Links PageLinks
	Meta  ResponseMeta
	Raw   json.RawMessage
}

type User struct {
	ID                      ID              `json:"id"`
	NodeID                  string          `json:"node_id"`
	Login                   string          `json:"login"`
	Name                    *string         `json:"name"`
	Email                   *string         `json:"email"`
	Company                 *string         `json:"company"`
	Blog                    *string         `json:"blog"`
	Location                *string         `json:"location"`
	Bio                     *string         `json:"bio"`
	TwitterUsername         *string         `json:"twitter_username"`
	AvatarURL               string          `json:"avatar_url"`
	URL                     string          `json:"url"`
	HTMLURL                 string          `json:"html_url"`
	Type                    string          `json:"type"`
	UserViewType            string          `json:"user_view_type"`
	SiteAdmin               bool            `json:"site_admin"`
	Hireable                *bool           `json:"hireable"`
	PublicRepos             int             `json:"public_repos"`
	PublicGists             int             `json:"public_gists"`
	Followers               int             `json:"followers"`
	Following               int             `json:"following"`
	CreatedAt               *time.Time      `json:"created_at"`
	UpdatedAt               *time.Time      `json:"updated_at"`
	PrivateGists            int             `json:"private_gists"`
	TotalPrivateRepos       int             `json:"total_private_repos"`
	OwnedPrivateRepos       int             `json:"owned_private_repos"`
	TwoFactorAuthentication *bool           `json:"two_factor_authentication"`
	Raw                     json.RawMessage `json:"-"`
}

func (value *User) UnmarshalJSON(data []byte) error {
	type wire User
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = User(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Repository struct {
	ID               ID              `json:"id"`
	NodeID           string          `json:"node_id"`
	Name             string          `json:"name"`
	FullName         string          `json:"full_name"`
	Owner            *User           `json:"owner"`
	Private          bool            `json:"private"`
	Fork             bool            `json:"fork"`
	Archived         bool            `json:"archived"`
	Disabled         bool            `json:"disabled"`
	Visibility       string          `json:"visibility"`
	URL              string          `json:"url"`
	HTMLURL          string          `json:"html_url"`
	CloneURL         string          `json:"clone_url"`
	SSHURL           string          `json:"ssh_url"`
	Description      *string         `json:"description"`
	Homepage         *string         `json:"homepage"`
	Language         *string         `json:"language"`
	DefaultBranch    string          `json:"default_branch"`
	Topics           []string        `json:"topics"`
	Size             int64           `json:"size"`
	ForksCount       int             `json:"forks_count"`
	StargazersCount  int             `json:"stargazers_count"`
	WatchersCount    int             `json:"watchers_count"`
	OpenIssuesCount  int             `json:"open_issues_count"`
	SubscribersCount int             `json:"subscribers_count"`
	NetworkCount     int             `json:"network_count"`
	HasIssues        bool            `json:"has_issues"`
	HasProjects      bool            `json:"has_projects"`
	HasWiki          bool            `json:"has_wiki"`
	HasPages         bool            `json:"has_pages"`
	HasDiscussions   bool            `json:"has_discussions"`
	CreatedAt        *time.Time      `json:"created_at"`
	UpdatedAt        *time.Time      `json:"updated_at"`
	PushedAt         *time.Time      `json:"pushed_at"`
	Raw              json.RawMessage `json:"-"`
}

func (value *Repository) UnmarshalJSON(data []byte) error {
	type wire Repository
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Repository(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// Label accepts both label objects and the string form allowed by the issue schema.
type Label struct {
	ID          ID              `json:"id"`
	NodeID      string          `json:"node_id"`
	URL         string          `json:"url"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	Color       *string         `json:"color"`
	Default     bool            `json:"default"`
	Raw         json.RawMessage `json:"-"`
}

func (value *Label) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var name string
		if err := json.Unmarshal(trimmed, &name); err != nil {
			return err
		}
		*value = Label{Name: name, Raw: append(json.RawMessage(nil), data...)}
		return nil
	}
	type wire Label
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Label(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type PullRequestLinks struct {
	MergedAt *time.Time `json:"merged_at"`
	DiffURL  *string    `json:"diff_url"`
	HTMLURL  *string    `json:"html_url"`
	PatchURL *string    `json:"patch_url"`
	URL      *string    `json:"url"`
}

type ReactionRollup struct {
	URL        string `json:"url"`
	TotalCount int    `json:"total_count"`
	PlusOne    int    `json:"+1"`
	MinusOne   int    `json:"-1"`
	Laugh      int    `json:"laugh"`
	Confused   int    `json:"confused"`
	Heart      int    `json:"heart"`
	Hooray     int    `json:"hooray"`
	Rocket     int    `json:"rocket"`
	Eyes       int    `json:"eyes"`
}

type Issue struct {
	ID                ID                `json:"id"`
	NodeID            string            `json:"node_id"`
	Number            ID                `json:"number"`
	URL               string            `json:"url"`
	RepositoryURL     string            `json:"repository_url"`
	HTMLURL           string            `json:"html_url"`
	CommentsURL       string            `json:"comments_url"`
	State             string            `json:"state"`
	StateReason       *string           `json:"state_reason"`
	Title             string            `json:"title"`
	Body              *string           `json:"body"`
	User              *User             `json:"user"`
	Labels            []Label           `json:"labels"`
	Assignee          *User             `json:"assignee"`
	Assignees         []User            `json:"assignees"`
	Locked            bool              `json:"locked"`
	ActiveLockReason  *string           `json:"active_lock_reason"`
	Comments          int               `json:"comments"`
	PullRequest       *PullRequestLinks `json:"pull_request"`
	AuthorAssociation string            `json:"author_association"`
	Reactions         *ReactionRollup   `json:"reactions"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	ClosedAt          *time.Time        `json:"closed_at"`
	Raw               json.RawMessage   `json:"-"`
}

func (value *Issue) UnmarshalJSON(data []byte) error {
	type wire Issue
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Issue(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type IssueComment struct {
	ID                ID              `json:"id"`
	NodeID            string          `json:"node_id"`
	URL               string          `json:"url"`
	HTMLURL           string          `json:"html_url"`
	IssueURL          string          `json:"issue_url"`
	Body              string          `json:"body"`
	User              *User           `json:"user"`
	AuthorAssociation string          `json:"author_association"`
	Reactions         *ReactionRollup `json:"reactions"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Raw               json.RawMessage `json:"-"`
}

func (value *IssueComment) UnmarshalJSON(data []byte) error {
	type wire IssueComment
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = IssueComment(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type RepositoryType string

const (
	RepositoryTypeAll     RepositoryType = "all"
	RepositoryTypeOwner   RepositoryType = "owner"
	RepositoryTypePublic  RepositoryType = "public"
	RepositoryTypePrivate RepositoryType = "private"
	RepositoryTypeMember  RepositoryType = "member"
)

type RepositoryVisibility string

const (
	RepositoryVisibilityAll     RepositoryVisibility = "all"
	RepositoryVisibilityPublic  RepositoryVisibility = "public"
	RepositoryVisibilityPrivate RepositoryVisibility = "private"
)

type RepositoryAffiliation string

const (
	RepositoryAffiliationOwner              RepositoryAffiliation = "owner"
	RepositoryAffiliationCollaborator       RepositoryAffiliation = "collaborator"
	RepositoryAffiliationOrganizationMember RepositoryAffiliation = "organization_member"
)

type RepositorySort string

const (
	RepositorySortCreated  RepositorySort = "created"
	RepositorySortUpdated  RepositorySort = "updated"
	RepositorySortPushed   RepositorySort = "pushed"
	RepositorySortFullName RepositorySort = "full_name"
)

type Direction string

const (
	DirectionAscending  Direction = "asc"
	DirectionDescending Direction = "desc"
)

type ListAuthenticatedRepositoriesRequest struct {
	Visibility  RepositoryVisibility
	Affiliation []RepositoryAffiliation
	Type        RepositoryType
	Sort        RepositorySort
	Direction   Direction
	Since       *time.Time
	Before      *time.Time
	PerPage     int
	Page        int
}

type ListUserRepositoriesRequest struct {
	Type      RepositoryType
	Sort      RepositorySort
	Direction Direction
	PerPage   int
	Page      int
}

type IssueState string

const (
	IssueStateOpen   IssueState = "open"
	IssueStateClosed IssueState = "closed"
	IssueStateAll    IssueState = "all"
)

type IssueSort string

const (
	IssueSortCreated  IssueSort = "created"
	IssueSortUpdated  IssueSort = "updated"
	IssueSortComments IssueSort = "comments"
)

type ListIssuesRequest struct {
	State     IssueState
	Labels    []string
	Sort      IssueSort
	Direction Direction
	Since     *time.Time
	PerPage   int
	Page      int
}

type ListIssueCommentsRequest struct {
	Since   *time.Time
	PerPage int
	Page    int
}

type ReadWorkflow interface {
	GetAuthenticatedUser(context.Context, ...socialhub.CallOption) (*User, ResponseMeta, error)
	GetUser(context.Context, string, ...socialhub.CallOption) (*User, ResponseMeta, error)
	ListAuthenticatedRepositories(context.Context, ListAuthenticatedRepositoriesRequest, ...socialhub.CallOption) (*Page[Repository], error)
	ListRepositoriesForUser(context.Context, string, ListUserRepositoriesRequest, ...socialhub.CallOption) (*Page[Repository], error)
	GetRepository(context.Context, string, string, ...socialhub.CallOption) (*Repository, ResponseMeta, error)
	ListIssues(context.Context, string, string, ListIssuesRequest, ...socialhub.CallOption) (*Page[Issue], error)
	GetIssue(context.Context, string, string, string, ...socialhub.CallOption) (*Issue, ResponseMeta, error)
	ListIssueComments(context.Context, string, string, string, ListIssueCommentsRequest, ...socialhub.CallOption) (*Page[IssueComment], error)
	GetIssueComment(context.Context, string, string, string, ...socialhub.CallOption) (*IssueComment, ResponseMeta, error)
}

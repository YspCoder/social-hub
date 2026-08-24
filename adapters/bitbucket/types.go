package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MinimumPageLength      = 1
	MaximumPageLength      = 100
	maxProviderObjectBytes = 8 << 20
)

// ID is a positive provider integer identifier.
type ID int64

func (value ID) String() string { return strconv.FormatInt(int64(value), 10) }

func (value *ID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > 19 || trimmed[0] == '0' {
		return fmt.Errorf("bitbucket: ID must be a positive int64 JSON number")
	}
	parsed, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("bitbucket: ID must be a positive int64 JSON number")
	}
	*value = ID(parsed)
	return nil
}

// ResponseMeta preserves request, caching, lifecycle, and documented scaled
// rate-limit headers. Bitbucket can omit rate headers outside scaled resources.
type ResponseMeta struct {
	RequestID          string
	RequestCount       string
	RateLimitLimit     string
	RateLimitResource  string
	RateLimitNearLimit string
	RetryAfter         string
	ETag               string
	Deprecation        string
	Sunset             string
	Warning            string
	RetryAfterDuration time.Duration
}

// PageOptions selects the first page length or replays an opaque provider next
// query. NextQuery is mutually exclusive with all first-page filters.
type PageOptions struct {
	PageLength int
	NextQuery  string
}

// Page is one Bitbucket page with provider pagination and bounded raw JSON.
type Page[T any] struct {
	Items         []T
	Size          *int
	PageNumber    *int
	PageLength    *int
	NextURL       string
	PreviousURL   string
	NextQuery     string
	PreviousQuery string
	Meta          ResponseMeta
	Raw           json.RawMessage
}

type Link struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type AccountLinks struct {
	Avatar Link `json:"avatar"`
}

type Account struct {
	Type          string          `json:"type"`
	DisplayName   string          `json:"display_name"`
	UUID          string          `json:"uuid"`
	CreatedOn     *time.Time      `json:"created_on"`
	AccountID     string          `json:"account_id"`
	AccountStatus string          `json:"account_status"`
	Has2FAEnabled *bool           `json:"has_2fa_enabled"`
	Nickname      string          `json:"nickname"`
	IsStaff       bool            `json:"is_staff"`
	Links         AccountLinks    `json:"links"`
	Raw           json.RawMessage `json:"-"`
}

func (value *Account) UnmarshalJSON(data []byte) error {
	type wire Account
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Account(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type WorkspaceBaseLinks struct {
	Avatar Link `json:"avatar"`
	Self   Link `json:"self"`
}

type WorkspaceBase struct {
	Type  string             `json:"type"`
	UUID  string             `json:"uuid"`
	Slug  string             `json:"slug"`
	Links WorkspaceBaseLinks `json:"links"`
	Raw   json.RawMessage    `json:"-"`
}

func (value *WorkspaceBase) UnmarshalJSON(data []byte) error {
	type wire WorkspaceBase
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = WorkspaceBase(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

// WorkspaceAccess combines a workspace with the current caller's admin flag.
type WorkspaceAccess struct {
	Type          string          `json:"type"`
	Administrator bool            `json:"administrator"`
	Workspace     WorkspaceBase   `json:"workspace"`
	Raw           json.RawMessage `json:"-"`
}

func (value *WorkspaceAccess) UnmarshalJSON(data []byte) error {
	type wire WorkspaceAccess
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = WorkspaceAccess(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type WorkspaceLinks struct {
	Avatar       Link `json:"avatar"`
	HTML         Link `json:"html"`
	Members      Link `json:"members"`
	Owners       Link `json:"owners"`
	Projects     Link `json:"projects"`
	Repositories Link `json:"repositories"`
	Snippets     Link `json:"snippets"`
	Self         Link `json:"self"`
}

type Workspace struct {
	Type              string          `json:"type"`
	UUID              string          `json:"uuid"`
	Name              string          `json:"name"`
	Slug              string          `json:"slug"`
	IsPrivate         bool            `json:"is_private"`
	IsPersonal        bool            `json:"is_personal"`
	IsPrivacyEnforced bool            `json:"is_privacy_enforced"`
	ForkingMode       string          `json:"forking_mode"`
	CreatedOn         *time.Time      `json:"created_on"`
	UpdatedOn         *time.Time      `json:"updated_on"`
	Links             WorkspaceLinks  `json:"links"`
	Raw               json.RawMessage `json:"-"`
}

func (value *Workspace) UnmarshalJSON(data []byte) error {
	type wire Workspace
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Workspace(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Project struct {
	Type        string     `json:"type"`
	UUID        string     `json:"uuid"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsPrivate   bool       `json:"is_private"`
	CreatedOn   *time.Time `json:"created_on"`
	UpdatedOn   *time.Time `json:"updated_on"`
}

type Branch struct {
	Type                 string   `json:"type"`
	Name                 string   `json:"name"`
	MergeStrategies      []string `json:"merge_strategies"`
	DefaultMergeStrategy string   `json:"default_merge_strategy"`
}

type RepositoryLinks struct {
	Self         Link   `json:"self"`
	HTML         Link   `json:"html"`
	Avatar       Link   `json:"avatar"`
	PullRequests Link   `json:"pullrequests"`
	Commits      Link   `json:"commits"`
	Forks        Link   `json:"forks"`
	Watchers     Link   `json:"watchers"`
	Downloads    Link   `json:"downloads"`
	Clone        []Link `json:"clone"`
	Hooks        Link   `json:"hooks"`
}

type Repository struct {
	Type        string          `json:"type"`
	UUID        string          `json:"uuid"`
	FullName    string          `json:"full_name"`
	Slug        string          `json:"slug"`
	IsPrivate   bool            `json:"is_private"`
	SCM         string          `json:"scm"`
	Owner       *Account        `json:"owner"`
	Workspace   *WorkspaceBase  `json:"workspace"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CreatedOn   *time.Time      `json:"created_on"`
	UpdatedOn   *time.Time      `json:"updated_on"`
	Size        int64           `json:"size"`
	Language    string          `json:"language"`
	HasIssues   bool            `json:"has_issues"`
	HasWiki     bool            `json:"has_wiki"`
	ForkPolicy  string          `json:"fork_policy"`
	Project     *Project        `json:"project"`
	MainBranch  *Branch         `json:"mainbranch"`
	Parent      *Repository     `json:"parent"`
	Links       RepositoryLinks `json:"links"`
	Raw         json.RawMessage `json:"-"`
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

type Markup struct {
	Raw    string `json:"raw"`
	Markup string `json:"markup"`
	HTML   string `json:"html"`
}

type RenderedPullRequest struct {
	Title       Markup `json:"title"`
	Description Markup `json:"description"`
	Reason      Markup `json:"reason"`
}

type PullRequestCommit struct {
	Hash string `json:"hash"`
}

type PullRequestBranch struct {
	Name                 string   `json:"name"`
	MergeStrategies      []string `json:"merge_strategies"`
	DefaultMergeStrategy string   `json:"default_merge_strategy"`
}

type PullRequestEndpoint struct {
	Repository *Repository       `json:"repository"`
	Branch     PullRequestBranch `json:"branch"`
	Commit     PullRequestCommit `json:"commit"`
}

type Participant struct {
	Type           string     `json:"type"`
	User           *Account   `json:"user"`
	Role           string     `json:"role"`
	Approved       bool       `json:"approved"`
	State          *string    `json:"state"`
	ParticipatedOn *time.Time `json:"participated_on"`
}

type PullRequestLinks struct {
	Self     Link `json:"self"`
	HTML     Link `json:"html"`
	Commits  Link `json:"commits"`
	Approve  Link `json:"approve"`
	Diff     Link `json:"diff"`
	Diffstat Link `json:"diffstat"`
	Comments Link `json:"comments"`
	Activity Link `json:"activity"`
	Merge    Link `json:"merge"`
	Decline  Link `json:"decline"`
}

type PullRequest struct {
	Type              string              `json:"type"`
	ID                ID                  `json:"id"`
	Title             string              `json:"title"`
	Description       string              `json:"description"`
	Rendered          RenderedPullRequest `json:"rendered"`
	Summary           Markup              `json:"summary"`
	State             string              `json:"state"`
	Author            *Account            `json:"author"`
	Source            PullRequestEndpoint `json:"source"`
	Destination       PullRequestEndpoint `json:"destination"`
	MergeCommit       *PullRequestCommit  `json:"merge_commit"`
	CommentCount      int                 `json:"comment_count"`
	TaskCount         int                 `json:"task_count"`
	CloseSourceBranch bool                `json:"close_source_branch"`
	ClosedBy          *Account            `json:"closed_by"`
	Reason            string              `json:"reason"`
	CreatedOn         *time.Time          `json:"created_on"`
	UpdatedOn         *time.Time          `json:"updated_on"`
	Reviewers         []Account           `json:"reviewers"`
	Participants      []Participant       `json:"participants"`
	Draft             bool                `json:"draft"`
	Queued            bool                `json:"queued"`
	Links             PullRequestLinks    `json:"links"`
	Raw               json.RawMessage     `json:"-"`
}

func (value *PullRequest) UnmarshalJSON(data []byte) error {
	type wire PullRequest
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = PullRequest(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type InlineComment struct {
	From      *int   `json:"from"`
	To        *int   `json:"to"`
	StartFrom *int   `json:"start_from"`
	StartTo   *int   `json:"start_to"`
	Path      string `json:"path"`
}

type CommentLinks struct {
	Self Link `json:"self"`
	HTML Link `json:"html"`
	Code Link `json:"code"`
}

type CommentResolution struct {
	Type      string     `json:"type"`
	User      *Account   `json:"user"`
	CreatedOn *time.Time `json:"created_on"`
}

type PullRequestComment struct {
	Type        string              `json:"type"`
	ID          ID                  `json:"id"`
	CreatedOn   *time.Time          `json:"created_on"`
	UpdatedOn   *time.Time          `json:"updated_on"`
	Content     Markup              `json:"content"`
	User        *Account            `json:"user"`
	Deleted     bool                `json:"deleted"`
	Parent      *PullRequestComment `json:"parent"`
	Inline      *InlineComment      `json:"inline"`
	Links       CommentLinks        `json:"links"`
	PullRequest *PullRequest        `json:"pullrequest"`
	Resolution  *CommentResolution  `json:"resolution"`
	Pending     bool                `json:"pending"`
	Raw         json.RawMessage     `json:"-"`
}

func (value *PullRequestComment) UnmarshalJSON(data []byte) error {
	type wire PullRequestComment
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = PullRequestComment(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type WorkspaceSort string

const (
	WorkspaceSortSlugAscending  WorkspaceSort = "slug"
	WorkspaceSortSlugDescending WorkspaceSort = "-slug"
)

type RepositoryRole string

const (
	RepositoryRoleAdmin       RepositoryRole = "admin"
	RepositoryRoleContributor RepositoryRole = "contributor"
	RepositoryRoleMember      RepositoryRole = "member"
	RepositoryRoleOwner       RepositoryRole = "owner"
)

type PullRequestState string

const (
	PullRequestStateOpen       PullRequestState = "OPEN"
	PullRequestStateMerged     PullRequestState = "MERGED"
	PullRequestStateDeclined   PullRequestState = "DECLINED"
	PullRequestStateSuperseded PullRequestState = "SUPERSEDED"
)

type ListWorkspacesRequest struct {
	Administrator *bool
	Sort          WorkspaceSort
	Page          PageOptions
}

type ListRepositoriesRequest struct {
	Role  RepositoryRole
	Query string
	Sort  string
	Page  PageOptions
}

type ListPullRequestsRequest struct {
	States []PullRequestState
	Query  string
	Sort   string
	Page   PageOptions
}

type ListPullRequestCommentsRequest struct {
	Query string
	Sort  string
	Page  PageOptions
}

// ReadWorkflow is the complete provider-native surface implemented here.
type ReadWorkflow interface {
	GetCurrentUser(context.Context, ...socialhub.CallOption) (*Account, ResponseMeta, error)
	ListWorkspaces(context.Context, ListWorkspacesRequest, ...socialhub.CallOption) (*Page[WorkspaceAccess], error)
	GetWorkspace(context.Context, string, ...socialhub.CallOption) (*Workspace, ResponseMeta, error)
	ListRepositories(context.Context, string, ListRepositoriesRequest, ...socialhub.CallOption) (*Page[Repository], error)
	GetRepository(context.Context, string, string, ...socialhub.CallOption) (*Repository, ResponseMeta, error)
	ListPullRequests(context.Context, string, string, ListPullRequestsRequest, ...socialhub.CallOption) (*Page[PullRequest], error)
	GetPullRequest(context.Context, string, string, ID, ...socialhub.CallOption) (*PullRequest, ResponseMeta, error)
	ListPullRequestComments(context.Context, string, string, ID, ListPullRequestCommentsRequest, ...socialhub.CallOption) (*Page[PullRequestComment], error)
	GetPullRequestComment(context.Context, string, string, ID, ID, ...socialhub.CallOption) (*PullRequestComment, ResponseMeta, error)
}

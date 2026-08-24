package gitlab

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

// ID preserves GitLab integer IDs and project-local IIDs as decimal strings.
type ID string

func (value *ID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if !validDecimalIDBytes(trimmed) {
		return fmt.Errorf("gitlab: ID must be a positive int64 JSON number")
	}
	*value = ID(string(trimmed))
	return nil
}

// ResponseMeta preserves GitLab request, rate, pagination, cache, and server
// metadata headers. Instance-dependent values remain available as text.
type ResponseMeta struct {
	RequestID           string
	GitLabMeta          string
	RateLimitLimit      string
	RateLimitName       string
	RateLimitObserved   string
	RateLimitRemaining  string
	RateLimitReset      string
	RateLimitResetTime  string
	RetryAfter          string
	ETag                string
	Link                string
	Page                string
	PerPage             string
	NextPage            string
	PreviousPage        string
	Total               string
	TotalPages          string
	RateLimitResetAt    *time.Time
	RateLimitResetAfter time.Duration
	RetryAfterDuration  time.Duration
}

// Pagination contains strictly parsed offset headers. Total fields may be nil
// because GitLab omits them for collections larger than 10,000 records.
type Pagination struct {
	Page         *int64
	PerPage      *int64
	NextPage     *int64
	PreviousPage *int64
	Total        *int64
	TotalPages   *int64
	Link         string
}

type Page[T any] struct {
	Items      []T
	Pagination Pagination
	Meta       ResponseMeta
	Raw        json.RawMessage
}

type User struct {
	ID                ID              `json:"id"`
	Username          string          `json:"username"`
	Name              string          `json:"name"`
	State             string          `json:"state"`
	Locked            bool            `json:"locked"`
	Bot               bool            `json:"bot"`
	AvatarURL         *string         `json:"avatar_url"`
	WebURL            string          `json:"web_url"`
	CreatedAt         *time.Time      `json:"created_at"`
	Bio               string          `json:"bio"`
	Location          *string         `json:"location"`
	PublicEmail       string          `json:"public_email"`
	Email             string          `json:"email"`
	LinkedIn          string          `json:"linkedin"`
	Twitter           string          `json:"twitter"`
	Discord           string          `json:"discord"`
	GitHub            string          `json:"github"`
	WebsiteURL        string          `json:"website_url"`
	Organization      string          `json:"organization"`
	JobTitle          string          `json:"job_title"`
	Pronouns          string          `json:"pronouns"`
	Followers         int64           `json:"followers"`
	Following         int64           `json:"following"`
	PreferredLanguage string          `json:"preferred_language"`
	Raw               json.RawMessage `json:"-"`
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

type Namespace struct {
	ID        ID      `json:"id"`
	Name      string  `json:"name"`
	Path      string  `json:"path"`
	Kind      string  `json:"kind"`
	FullPath  string  `json:"full_path"`
	ParentID  *ID     `json:"parent_id"`
	AvatarURL *string `json:"avatar_url"`
	WebURL    string  `json:"web_url"`
}

type Project struct {
	ID                   ID              `json:"id"`
	Description          *string         `json:"description"`
	Name                 string          `json:"name"`
	NameWithNamespace    string          `json:"name_with_namespace"`
	Path                 string          `json:"path"`
	PathWithNamespace    string          `json:"path_with_namespace"`
	CreatedAt            *time.Time      `json:"created_at"`
	LastActivityAt       *time.Time      `json:"last_activity_at"`
	DefaultBranch        *string         `json:"default_branch"`
	Topics               []string        `json:"topics"`
	SSHURLToRepo         string          `json:"ssh_url_to_repo"`
	HTTPURLToRepo        string          `json:"http_url_to_repo"`
	WebURL               string          `json:"web_url"`
	ReadmeURL            *string         `json:"readme_url"`
	AvatarURL            *string         `json:"avatar_url"`
	ForksCount           int64           `json:"forks_count"`
	StarCount            int64           `json:"star_count"`
	Visibility           string          `json:"visibility"`
	Archived             bool            `json:"archived"`
	EmptyRepo            bool            `json:"empty_repo"`
	IssuesEnabled        bool            `json:"issues_enabled"`
	MergeRequestsEnabled bool            `json:"merge_requests_enabled"`
	WikiEnabled          bool            `json:"wiki_enabled"`
	Namespace            *Namespace      `json:"namespace"`
	Owner                *User           `json:"owner"`
	Raw                  json.RawMessage `json:"-"`
}

func (value *Project) UnmarshalJSON(data []byte) error {
	type wire Project
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Project(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type IssueReferences struct {
	Short    string `json:"short"`
	Relative string `json:"relative"`
	Full     string `json:"full"`
}

type Milestone struct {
	ID          ID              `json:"id"`
	IID         ID              `json:"iid"`
	ProjectID   *ID             `json:"project_id"`
	GroupID     *ID             `json:"group_id"`
	Title       string          `json:"title"`
	Description *string         `json:"description"`
	State       string          `json:"state"`
	CreatedAt   *time.Time      `json:"created_at"`
	UpdatedAt   *time.Time      `json:"updated_at"`
	DueDate     *string         `json:"due_date"`
	Raw         json.RawMessage `json:"-"`
}

func (value *Milestone) UnmarshalJSON(data []byte) error {
	type wire Milestone
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Milestone(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type Issue struct {
	ID               ID              `json:"id"`
	IID              ID              `json:"iid"`
	ProjectID        ID              `json:"project_id"`
	Title            string          `json:"title"`
	Description      *string         `json:"description"`
	State            string          `json:"state"`
	IssueType        string          `json:"issue_type"`
	Severity         string          `json:"severity"`
	Author           *User           `json:"author"`
	Assignees        []User          `json:"assignees"`
	Labels           []string        `json:"labels"`
	Milestone        *Milestone      `json:"milestone"`
	Upvotes          int64           `json:"upvotes"`
	Downvotes        int64           `json:"downvotes"`
	UserNotesCount   int64           `json:"user_notes_count"`
	Confidential     bool            `json:"confidential"`
	DiscussionLocked *bool           `json:"discussion_locked"`
	Imported         bool            `json:"imported"`
	ImportedFrom     string          `json:"imported_from"`
	Weight           *int64          `json:"weight"`
	DueDate          *string         `json:"due_date"`
	WebURL           string          `json:"web_url"`
	References       IssueReferences `json:"references"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	ClosedAt         *time.Time      `json:"closed_at"`
	Raw              json.RawMessage `json:"-"`
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

type Note struct {
	ID           ID              `json:"id"`
	Body         string          `json:"body"`
	Author       *User           `json:"author"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	System       bool            `json:"system"`
	NoteableID   ID              `json:"noteable_id"`
	NoteableIID  ID              `json:"noteable_iid"`
	NoteableType string          `json:"noteable_type"`
	ProjectID    ID              `json:"project_id"`
	Resolvable   bool            `json:"resolvable"`
	Confidential bool            `json:"confidential"`
	Internal     bool            `json:"internal"`
	Imported     bool            `json:"imported"`
	ImportedFrom string          `json:"imported_from"`
	Raw          json.RawMessage `json:"-"`
}

func (value *Note) UnmarshalJSON(data []byte) error {
	type wire Note
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*value = Note(decoded)
	value.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

type ProjectVisibility string

const (
	ProjectVisibilityPrivate  ProjectVisibility = "private"
	ProjectVisibilityInternal ProjectVisibility = "internal"
	ProjectVisibilityPublic   ProjectVisibility = "public"
)

type ProjectOrder string

const (
	ProjectOrderID             ProjectOrder = "id"
	ProjectOrderName           ProjectOrder = "name"
	ProjectOrderPath           ProjectOrder = "path"
	ProjectOrderCreatedAt      ProjectOrder = "created_at"
	ProjectOrderUpdatedAt      ProjectOrder = "updated_at"
	ProjectOrderLastActivityAt ProjectOrder = "last_activity_at"
	ProjectOrderStarCount      ProjectOrder = "star_count"
)

type ListProjectsRequest struct {
	Visibility ProjectVisibility
	OrderBy    ProjectOrder
	Sort       SortDirection
	Search     string
	Membership bool
	Owned      bool
	Archived   *bool
	PerPage    int
	Page       int
}

type IssueState string

const (
	IssueStateOpened IssueState = "opened"
	IssueStateClosed IssueState = "closed"
	IssueStateAll    IssueState = "all"
)

type IssueOrder string

const (
	IssueOrderCreatedAt IssueOrder = "created_at"
	IssueOrderUpdatedAt IssueOrder = "updated_at"
)

type ListProjectIssuesRequest struct {
	State         IssueState
	Labels        []string
	OrderBy       IssueOrder
	Sort          SortDirection
	Search        string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time
	PerPage       int
	Page          int
}

type NoteActivityFilter string

const (
	NoteActivityAll      NoteActivityFilter = "all_notes"
	NoteActivityComments NoteActivityFilter = "only_comments"
	NoteActivitySystem   NoteActivityFilter = "only_activity"
)

type NoteOrder string

const (
	NoteOrderCreatedAt NoteOrder = "created_at"
	NoteOrderUpdatedAt NoteOrder = "updated_at"
)

type ListIssueNotesRequest struct {
	ActivityFilter NoteActivityFilter
	OrderBy        NoteOrder
	Sort           SortDirection
	PerPage        int
	Page           int
}

type ReadWorkflow interface {
	GetAuthenticatedUser(context.Context, ...socialhub.CallOption) (*User, ResponseMeta, error)
	GetUser(context.Context, string, ...socialhub.CallOption) (*User, ResponseMeta, error)
	ListProjects(context.Context, ListProjectsRequest, ...socialhub.CallOption) (*Page[Project], error)
	GetProject(context.Context, string, ...socialhub.CallOption) (*Project, ResponseMeta, error)
	ListProjectIssues(context.Context, string, ListProjectIssuesRequest, ...socialhub.CallOption) (*Page[Issue], error)
	GetProjectIssue(context.Context, string, string, ...socialhub.CallOption) (*Issue, ResponseMeta, error)
	ListIssueNotes(context.Context, string, string, ListIssueNotesRequest, ...socialhub.CallOption) (*Page[Note], error)
	GetIssueNote(context.Context, string, string, string, ...socialhub.CallOption) (*Note, ResponseMeta, error)
}

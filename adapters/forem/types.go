package forem

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type stringList []string

func (list *stringList) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*list = cleanTags(values)
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*list = cleanTags(strings.Split(value, ","))
	return nil
}

type wireUser struct {
	TypeOf          string          `json:"type_of"`
	ID              int64           `json:"id"`
	UserID          int64           `json:"user_id"`
	Username        string          `json:"username"`
	Name            string          `json:"name"`
	Summary         *string         `json:"summary"`
	TwitterUsername *string         `json:"twitter_username"`
	GitHubUsername  *string         `json:"github_username"`
	Email           *string         `json:"email"`
	WebsiteURL      *string         `json:"website_url"`
	Location        *string         `json:"location"`
	JoinedAt        string          `json:"joined_at"`
	ProfileImage    string          `json:"profile_image"`
	ProfileImage90  string          `json:"profile_image_90"`
	FollowersCount  int             `json:"followers_count"`
	Raw             json.RawMessage `json:"-"`
}

func (user *wireUser) UnmarshalJSON(data []byte) error {
	type alias wireUser
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*user = wireUser(decoded)
	user.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (user wireUser) identifier() int64 {
	if user.ID > 0 {
		return user.ID
	}
	return user.UserID
}

type wireArticle struct {
	TypeOf                 string          `json:"type_of"`
	ID                     int64           `json:"id"`
	Title                  string          `json:"title"`
	Description            string          `json:"description"`
	CoverImage             string          `json:"cover_image"`
	SocialImage            string          `json:"social_image"`
	TagList                stringList      `json:"tag_list"`
	Tags                   stringList      `json:"tags"`
	Slug                   string          `json:"slug"`
	Path                   string          `json:"path"`
	URL                    string          `json:"url"`
	CanonicalURL           string          `json:"canonical_url"`
	CommentsCount          int             `json:"comments_count"`
	PositiveReactionsCount int             `json:"positive_reactions_count"`
	PublicReactionsCount   int             `json:"public_reactions_count"`
	PageViewsCount         int             `json:"page_views_count"`
	CreatedAt              *time.Time      `json:"created_at"`
	EditedAt               *time.Time      `json:"edited_at"`
	CrosspostedAt          *time.Time      `json:"crossposted_at"`
	PublishedAt            *time.Time      `json:"published_at"`
	PublishedTimestamp     *time.Time      `json:"published_timestamp"`
	BodyHTML               string          `json:"body_html"`
	BodyMarkdown           string          `json:"body_markdown"`
	Published              bool            `json:"published"`
	ReadingTimeMinutes     int             `json:"reading_time_minutes"`
	User                   wireUser        `json:"user"`
	Raw                    json.RawMessage `json:"-"`
}

func (article *wireArticle) UnmarshalJSON(data []byte) error {
	type alias wireArticle
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*article = wireArticle(decoded)
	article.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func (article wireArticle) tags() []string {
	if len(article.TagList) > 0 {
		return append([]string(nil), article.TagList...)
	}
	return append([]string(nil), article.Tags...)
}

type wireComment struct {
	TypeOf    string          `json:"type_of"`
	IDCode    string          `json:"id_code"`
	CreatedAt *time.Time      `json:"created_at"`
	BodyHTML  string          `json:"body_html"`
	User      wireUser        `json:"user"`
	Children  []wireComment   `json:"children"`
	Raw       json.RawMessage `json:"-"`
}

func (comment *wireComment) UnmarshalJSON(data []byte) error {
	type alias wireComment
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*comment = wireComment(decoded)
	comment.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type articleEnvelope struct {
	Article articleAttributes `json:"article"`
}

type articleAttributes struct {
	Title          *string `json:"title,omitempty"`
	BodyMarkdown   *string `json:"body_markdown,omitempty"`
	Published      *bool   `json:"published,omitempty"`
	Series         *string `json:"series,omitempty"`
	MainImage      *string `json:"main_image,omitempty"`
	CanonicalURL   *string `json:"canonical_url,omitempty"`
	Description    *string `json:"description,omitempty"`
	Tags           *string `json:"tags,omitempty"`
	OrganizationID *int64  `json:"organization_id,omitempty"`
}

// ArticleState selects one authenticated-account article collection.
type ArticleState string

const (
	ArticleStateAll         ArticleState = "all"
	ArticleStatePublished   ArticleState = "published"
	ArticleStateUnpublished ArticleState = "unpublished"
)

// Article preserves Forem metadata alongside the common Post representation.
type Article struct {
	Post                   socialhub.Post
	Title                  string
	Description            string
	Slug                   string
	CanonicalURL           string
	Tags                   []string
	BodyMarkdown           string
	BodyHTML               string
	Published              bool
	ReadingTimeMinutes     int
	CommentsCount          int
	PositiveReactionsCount int
	PublicReactionsCount   int
	PageViewsCount         int
	Raw                    json.RawMessage
}

// CreateArticleRequest creates a Forem draft or published Article.
type CreateArticleRequest struct {
	Title          string
	BodyMarkdown   string
	Published      bool
	Series         string
	MainImageURL   string
	CanonicalURL   string
	Description    string
	Tags           []string
	OrganizationID string
}

// UpdateArticleRequest patches only non-nil fields. The V1 API also accepts
// JSON null for removing nullable metadata; this typed request does not encode
// that state. Update BodyMarkdown with revised front matter when removal is
// required.
type UpdateArticleRequest struct {
	Title          *string
	BodyMarkdown   *string
	Published      *bool
	Series         *string
	MainImageURL   *string
	CanonicalURL   *string
	Description    *string
	Tags           *[]string
	OrganizationID *string
}

// ArticleWorkflow exposes Forem Article semantics that require fields absent
// from socialhub.CreatePostRequest.
type ArticleWorkflow interface {
	CreateArticle(context.Context, CreateArticleRequest, ...socialhub.CallOption) (*Article, error)
	GetArticle(context.Context, string, ...socialhub.CallOption) (*Article, error)
	UpdateArticle(context.Context, string, UpdateArticleRequest, ...socialhub.CallOption) (*Article, error)
	UnpublishArticle(context.Context, string, string, ...socialhub.CallOption) error
	ListMyArticles(context.Context, ArticleState, string, int, ...socialhub.CallOption) (socialhub.Page[Article], error)
}

// ReactionCategory is one reaction documented by Forem API V1.
type ReactionCategory string

const (
	ReactionLike          ReactionCategory = "like"
	ReactionUnicorn       ReactionCategory = "unicorn"
	ReactionExplodingHead ReactionCategory = "exploding_head"
	ReactionRaisedHands   ReactionCategory = "raised_hands"
	ReactionFire          ReactionCategory = "fire"
)

// ReactableType selects the Forem resource receiving a reaction.
type ReactableType string

const (
	ReactableArticle ReactableType = "Article"
	ReactableComment ReactableType = "Comment"
	ReactableUser    ReactableType = "User"
)

// ForemReactionRequest targets an Article, Comment, or User.
type ForemReactionRequest struct {
	Category ReactionCategory
	TargetID string
	Type     ReactableType
}

// ReactionWorkflow exposes all documented categories and an explicit toggle.
type ReactionWorkflow interface {
	CreateForemReaction(context.Context, ForemReactionRequest, ...socialhub.CallOption) error
	ToggleForemReaction(context.Context, ForemReactionRequest, ...socialhub.CallOption) error
}

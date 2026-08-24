package producthunt

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type PostsOrder string

const (
	PostsOrderFeaturedAt PostsOrder = "FEATURED_AT"
	PostsOrderNewest     PostsOrder = "NEWEST"
	PostsOrderRanking    PostsOrder = "RANKING"
	PostsOrderVotes      PostsOrder = "VOTES"
)

type TopicsOrder string

const (
	TopicsOrderFollowersCount TopicsOrder = "FOLLOWERS_COUNT"
	TopicsOrderNewest         TopicsOrder = "NEWEST"
)

type CollectionsOrder string

const (
	CollectionsOrderFeaturedAt     CollectionsOrder = "FEATURED_AT"
	CollectionsOrderFollowersCount CollectionsOrder = "FOLLOWERS_COUNT"
	CollectionsOrderNewest         CollectionsOrder = "NEWEST"
)

type CommentsOrder string

const (
	CommentsOrderNewest     CommentsOrder = "NEWEST"
	CommentsOrderVotesCount CommentsOrder = "VOTES_COUNT"
)

// Pagination represents one direction of a Relay cursor request. Product Hunt
// does not publish a numeric maximum for First or Last.
type Pagination struct {
	First  int
	After  string
	Last   int
	Before string
}

type ObjectLookup struct {
	ID   string
	Slug string
}

type UserLookup struct {
	ID       string
	Username string
}

type ListPostsRequest struct {
	Page         Pagination
	Featured     *bool
	Order        PostsOrder
	PostedAfter  time.Time
	PostedBefore time.Time
	Topic        string
	TwitterURL   string
	URL          string
}

type ListTopicsRequest struct {
	Page             Pagination
	FollowedByUserID string
	Order            TopicsOrder
	Query            string
}

type ListCollectionsRequest struct {
	Page     Pagination
	Featured *bool
	Order    CollectionsOrder
	PostID   string
	UserID   string
}

type ListPostCommentsRequest struct {
	Post  ObjectLookup
	Page  Pagination
	Order CommentsOrder
}

// ReadWorkflow is the bounded Product Hunt API v2 read-only surface.
type ReadWorkflow interface {
	ListPosts(context.Context, ListPostsRequest, ...socialhub.CallOption) (PostsResponse, error)
	GetPost(context.Context, ObjectLookup, ...socialhub.CallOption) (PostResponse, error)
	ListTopics(context.Context, ListTopicsRequest, ...socialhub.CallOption) (TopicsResponse, error)
	GetTopic(context.Context, ObjectLookup, ...socialhub.CallOption) (TopicResponse, error)
	ListCollections(context.Context, ListCollectionsRequest, ...socialhub.CallOption) (CollectionsResponse, error)
	GetCollection(context.Context, ObjectLookup, ...socialhub.CallOption) (CollectionResponse, error)
	ListPostComments(context.Context, ListPostCommentsRequest, ...socialhub.CallOption) (CommentsResponse, error)
	GetComment(context.Context, string, ...socialhub.CallOption) (CommentResponse, error)
	GetUser(context.Context, UserLookup, ...socialhub.CallOption) (UserResponse, error)
	GetViewer(context.Context, ...socialhub.CallOption) (UserResponse, error)
}

type ResponseMeta struct {
	RequestID          string
	RateLimitLimit     string
	RateLimitRemaining string
	RateLimitReset     string
}

type PageInfo struct {
	EndCursor       *string `json:"endCursor"`
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *string `json:"startCursor"`
}

type Edge[T any] struct {
	Cursor string `json:"cursor"`
	Node   T      `json:"node"`
}

type Connection[T any] struct {
	Edges      []Edge[T] `json:"edges"`
	PageInfo   PageInfo  `json:"pageInfo"`
	TotalCount int       `json:"totalCount"`
}

type ProductLink struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type Media struct {
	Type     string  `json:"type"`
	URL      string  `json:"url"`
	VideoURL *string `json:"videoUrl"`
}

type User struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Username        string     `json:"username"`
	Headline        *string    `json:"headline"`
	CreatedAt       *time.Time `json:"createdAt"`
	FollowersCount  int        `json:"followersCount"`
	FollowingCount  int        `json:"followingCount"`
	IsFollowing     bool       `json:"isFollowing"`
	IsMaker         bool       `json:"isMaker"`
	IsViewer        bool       `json:"isViewer"`
	ProfileImage    *string    `json:"profileImage"`
	CoverImage      *string    `json:"coverImage"`
	TwitterUsername *string    `json:"twitterUsername"`
	URL             string     `json:"url"`
	WebsiteURL      *string    `json:"websiteUrl"`
}

type Post struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Slug          string        `json:"slug"`
	Tagline       string        `json:"tagline"`
	Description   *string       `json:"description"`
	CreatedAt     time.Time     `json:"createdAt"`
	FeaturedAt    *time.Time    `json:"featuredAt"`
	ScheduledAt   *time.Time    `json:"scheduledAt"`
	DailyRank     *int          `json:"dailyRank"`
	WeeklyRank    *int          `json:"weeklyRank"`
	MonthlyRank   *int          `json:"monthlyRank"`
	YearlyRank    *int          `json:"yearlyRank"`
	CommentsCount int           `json:"commentsCount"`
	VotesCount    int           `json:"votesCount"`
	ReviewsCount  int           `json:"reviewsCount"`
	ReviewsRating float64       `json:"reviewsRating"`
	IsCollected   bool          `json:"isCollected"`
	IsVoted       bool          `json:"isVoted"`
	Makers        []User        `json:"makers"`
	Media         []Media       `json:"media"`
	ProductLinks  []ProductLink `json:"productLinks"`
	Thumbnail     *Media        `json:"thumbnail"`
	URL           string        `json:"url"`
	Website       string        `json:"website"`
	User          User          `json:"user"`
	UserID        string        `json:"userId"`
}

type Topic struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"createdAt"`
	FollowersCount int       `json:"followersCount"`
	PostsCount     int       `json:"postsCount"`
	IsFollowing    bool      `json:"isFollowing"`
	Image          *string   `json:"image"`
	URL            string    `json:"url"`
}

type Collection struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Tagline        string     `json:"tagline"`
	Description    *string    `json:"description"`
	CoverImage     *string    `json:"coverImage"`
	CreatedAt      time.Time  `json:"createdAt"`
	FeaturedAt     *time.Time `json:"featuredAt"`
	FollowersCount int        `json:"followersCount"`
	IsFollowing    bool       `json:"isFollowing"`
	URL            string     `json:"url"`
	User           User       `json:"user"`
	UserID         string     `json:"userId"`
}

type Comment struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	URL        string    `json:"url"`
	Parent     *Comment  `json:"parent"`
	ParentID   *string   `json:"parentId"`
	User       User      `json:"user"`
	UserID     string    `json:"userId"`
	IsVoted    bool      `json:"isVoted"`
	VotesCount int       `json:"votesCount"`
}

type PostsResponse struct {
	Posts Connection[Post] `json:"posts"`
	Meta  ResponseMeta     `json:"-"`
	Raw   json.RawMessage  `json:"-"`
}

type PostResponse struct {
	Post *Post           `json:"post"`
	Meta ResponseMeta    `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

type TopicsResponse struct {
	Topics Connection[Topic] `json:"topics"`
	Meta   ResponseMeta      `json:"-"`
	Raw    json.RawMessage   `json:"-"`
}

type TopicResponse struct {
	Topic *Topic          `json:"topic"`
	Meta  ResponseMeta    `json:"-"`
	Raw   json.RawMessage `json:"-"`
}

type CollectionsResponse struct {
	Collections Connection[Collection] `json:"collections"`
	Meta        ResponseMeta           `json:"-"`
	Raw         json.RawMessage        `json:"-"`
}

type CollectionResponse struct {
	Collection *Collection     `json:"collection"`
	Meta       ResponseMeta    `json:"-"`
	Raw        json.RawMessage `json:"-"`
}

type CommentsResponse struct {
	Comments Connection[Comment] `json:"comments"`
	Meta     ResponseMeta        `json:"-"`
	Raw      json.RawMessage     `json:"-"`
}

type CommentResponse struct {
	Comment *Comment        `json:"comment"`
	Meta    ResponseMeta    `json:"-"`
	Raw     json.RawMessage `json:"-"`
}

type UserResponse struct {
	User *User           `json:"user"`
	Meta ResponseMeta    `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

var _ ReadWorkflow = (*Client)(nil)

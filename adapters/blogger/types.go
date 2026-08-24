package blogger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const maxProviderObjectBytes = 8 << 20

type ViewType string

const (
	ViewUnspecified ViewType = "VIEW_TYPE_UNSPECIFIED"
	ViewReader      ViewType = "READER"
	ViewAuthor      ViewType = "AUTHOR"
	ViewAdmin       ViewType = "ADMIN"
)

type BlogStatus string

const (
	BlogStatusLive    BlogStatus = "LIVE"
	BlogStatusDeleted BlogStatus = "DELETED"
)

type PostStatus string

const (
	PostStatusLive        PostStatus = "LIVE"
	PostStatusDraft       PostStatus = "DRAFT"
	PostStatusScheduled   PostStatus = "SCHEDULED"
	PostStatusSoftTrashed PostStatus = "SOFT_TRASHED"
)

type CommentStatus string

const (
	CommentStatusLive    CommentStatus = "LIVE"
	CommentStatusEmptied CommentStatus = "EMPTIED"
	CommentStatusPending CommentStatus = "PENDING"
	CommentStatusSpam    CommentStatus = "SPAM"
)

type PageStatus string

const (
	PageStatusLive        PageStatus = "LIVE"
	PageStatusDraft       PageStatus = "DRAFT"
	PageStatusSoftTrashed PageStatus = "SOFT_TRASHED"
)

type PostOrder string

const (
	PostOrderUnspecified PostOrder = "ORDER_BY_UNSPECIFIED"
	PostOrderPublished   PostOrder = "PUBLISHED"
	PostOrderUpdated     PostOrder = "UPDATED"
)

type SortOption string

const (
	SortUnspecified SortOption = "SORT_OPTION_UNSPECIFIED"
	SortDescending  SortOption = "DESCENDING"
	SortAscending   SortOption = "ASCENDING"
)

type GetBlogRequest struct {
	BlogID   string
	MaxPosts uint32
	View     ViewType
}

type GetBlogByURLRequest struct {
	URL  string
	View ViewType
}

type ListBlogsByUserRequest struct {
	UserID        string
	Status        BlogStatus
	FetchUserInfo *bool
	Role          ViewType
	View          ViewType
}

type GetPostRequest struct {
	BlogID      string
	PostID      string
	FetchBody   *bool
	FetchImages *bool
	MaxComments uint32
	View        ViewType
}

type GetPostByPathRequest struct {
	BlogID      string
	Path        string
	MaxComments uint32
	View        ViewType
}

type ListPostsRequest struct {
	BlogID      string
	FetchBodies *bool
	FetchImages *bool
	PageToken   string
	Status      PostStatus
	MaxResults  uint32
	StartDate   string
	EndDate     string
	View        ViewType
	OrderBy     PostOrder
	Sort        SortOption
	Labels      []string
}

type SearchPostsRequest struct {
	BlogID      string
	Query       string
	OrderBy     PostOrder
	FetchBodies *bool
}

type GetCommentRequest struct {
	BlogID    string
	PostID    string
	CommentID string
	View      ViewType
}

type ListCommentsRequest struct {
	BlogID      string
	PostID      string
	FetchBodies *bool
	PageToken   string
	Status      CommentStatus
	StartDate   string
	EndDate     string
	MaxResults  uint32
	View        ViewType
}

type ListBlogCommentsRequest struct {
	BlogID      string
	FetchBodies *bool
	PageToken   string
	Status      CommentStatus
	StartDate   string
	EndDate     string
	MaxResults  uint32
}

type GetPageRequest struct {
	BlogID string
	PageID string
	View   ViewType
}

type ListPagesRequest struct {
	BlogID      string
	FetchBodies *bool
	PageToken   string
	Status      PageStatus
	MaxResults  uint32
	View        ViewType
}

type BlogsWorkflow interface {
	GetBlog(context.Context, GetBlogRequest, ...socialhub.CallOption) (Blog, error)
	GetBlogByURL(context.Context, GetBlogByURLRequest, ...socialhub.CallOption) (Blog, error)
	ListBlogsByUser(context.Context, ListBlogsByUserRequest, ...socialhub.CallOption) (BlogList, error)
}

type PostsWorkflow interface {
	GetPost(context.Context, GetPostRequest, ...socialhub.CallOption) (Post, error)
	GetPostByPath(context.Context, GetPostByPathRequest, ...socialhub.CallOption) (Post, error)
	ListPosts(context.Context, ListPostsRequest, ...socialhub.CallOption) (PostList, error)
	SearchPosts(context.Context, SearchPostsRequest, ...socialhub.CallOption) (PostList, error)
}

type CommentsWorkflow interface {
	GetComment(context.Context, GetCommentRequest, ...socialhub.CallOption) (Comment, error)
	ListComments(context.Context, ListCommentsRequest, ...socialhub.CallOption) (CommentList, error)
	ListBlogComments(context.Context, ListBlogCommentsRequest, ...socialhub.CallOption) (CommentList, error)
}

type PagesWorkflow interface {
	GetPage(context.Context, GetPageRequest, ...socialhub.CallOption) (Page, error)
	ListPages(context.Context, ListPagesRequest, ...socialhub.CallOption) (PageList, error)
}

type ReadWorkflow interface {
	BlogsWorkflow
	PostsWorkflow
	CommentsWorkflow
	PagesWorkflow
}

type ResponseMeta struct {
	RequestID        string
	TraceContext     string
	ETag             string
	RetryAfterHeader string
	RetryAfter       time.Duration
	QuotaHeaders     map[string]string
}

type BlogLocale struct {
	Language string `json:"language"`
	Country  string `json:"country"`
	Variant  string `json:"variant"`
}

type ResourceRef struct {
	ID string `json:"id"`
}

type AuthorImage struct {
	URL string `json:"url"`
}

type Author struct {
	ID          string      `json:"id"`
	DisplayName string      `json:"displayName"`
	URL         string      `json:"url"`
	Image       AuthorImage `json:"image"`
}

type BlogPostContainer struct {
	TotalItems int    `json:"totalItems"`
	SelfLink   string `json:"selfLink"`
	Items      []Post `json:"items"`
}

type BlogPageContainer struct {
	TotalItems int    `json:"totalItems"`
	SelfLink   string `json:"selfLink"`
}

type Blog struct {
	Kind        string            `json:"kind"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Published   string            `json:"published"`
	Updated     string            `json:"updated"`
	URL         string            `json:"url"`
	SelfLink    string            `json:"selfLink"`
	Status      BlogStatus        `json:"status"`
	Locale      BlogLocale        `json:"locale"`
	Posts       BlogPostContainer `json:"posts"`
	Pages       BlogPageContainer `json:"pages"`
	Meta        ResponseMeta      `json:"-"`
	Raw         json.RawMessage   `json:"-"`
}

func (value *Blog) UnmarshalJSON(data []byte) error {
	type wire Blog
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Blog(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type BlogList struct {
	Kind  string          `json:"kind"`
	Items []Blog          `json:"items"`
	Meta  ResponseMeta    `json:"-"`
	Raw   json.RawMessage `json:"-"`
}

func (value *BlogList) UnmarshalJSON(data []byte) error {
	type wire BlogList
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = BlogList(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PostLocation struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"lat"`
	Longitude *float64 `json:"lng"`
	Span      string   `json:"span"`
}

type PostImage struct {
	URL string `json:"url"`
}

type PostReplies struct {
	TotalItems string    `json:"totalItems"`
	SelfLink   string    `json:"selfLink"`
	Items      []Comment `json:"items"`
}

type Post struct {
	Kind           string          `json:"kind"`
	ID             string          `json:"id"`
	Blog           ResourceRef     `json:"blog"`
	Published      string          `json:"published"`
	Updated        string          `json:"updated"`
	Trashed        string          `json:"trashed"`
	URL            string          `json:"url"`
	SelfLink       string          `json:"selfLink"`
	Title          string          `json:"title"`
	TitleLink      string          `json:"titleLink"`
	Content        string          `json:"content"`
	Author         Author          `json:"author"`
	Replies        PostReplies     `json:"replies"`
	Labels         []string        `json:"labels"`
	Status         PostStatus      `json:"status"`
	Location       *PostLocation   `json:"location"`
	Images         []PostImage     `json:"images"`
	ReaderComments string          `json:"readerComments"`
	ETag           string          `json:"etag"`
	Meta           ResponseMeta    `json:"-"`
	Raw            json.RawMessage `json:"-"`
}

func (value *Post) UnmarshalJSON(data []byte) error {
	type wire Post
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Post(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PostList struct {
	Kind          string          `json:"kind"`
	Items         []Post          `json:"items"`
	NextPageToken string          `json:"nextPageToken"`
	PrevPageToken string          `json:"prevPageToken"`
	ETag          string          `json:"etag"`
	Meta          ResponseMeta    `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func (value *PostList) UnmarshalJSON(data []byte) error {
	type wire PostList
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PostList(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Comment struct {
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	Blog      ResourceRef     `json:"blog"`
	Post      ResourceRef     `json:"post"`
	InReplyTo *ResourceRef    `json:"inReplyTo"`
	Published string          `json:"published"`
	Updated   string          `json:"updated"`
	Content   string          `json:"content"`
	Author    Author          `json:"author"`
	SelfLink  string          `json:"selfLink"`
	Status    CommentStatus   `json:"status"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (value *Comment) UnmarshalJSON(data []byte) error {
	type wire Comment
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Comment(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type CommentList struct {
	Kind          string          `json:"kind"`
	Items         []Comment       `json:"items"`
	NextPageToken string          `json:"nextPageToken"`
	PrevPageToken string          `json:"prevPageToken"`
	ETag          string          `json:"etag"`
	Meta          ResponseMeta    `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func (value *CommentList) UnmarshalJSON(data []byte) error {
	type wire CommentList
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = CommentList(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type Page struct {
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	Blog      ResourceRef     `json:"blog"`
	Published string          `json:"published"`
	Updated   string          `json:"updated"`
	Trashed   string          `json:"trashed"`
	URL       string          `json:"url"`
	SelfLink  string          `json:"selfLink"`
	Title     string          `json:"title"`
	Content   string          `json:"content"`
	Author    Author          `json:"author"`
	Status    PageStatus      `json:"status"`
	ETag      string          `json:"etag"`
	Meta      ResponseMeta    `json:"-"`
	Raw       json.RawMessage `json:"-"`
}

func (value *Page) UnmarshalJSON(data []byte) error {
	type wire Page
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = Page(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type PageList struct {
	Kind          string          `json:"kind"`
	Items         []Page          `json:"items"`
	NextPageToken string          `json:"nextPageToken"`
	ETag          string          `json:"etag"`
	Meta          ResponseMeta    `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func (value *PageList) UnmarshalJSON(data []byte) error {
	type wire PageList
	var decoded wire
	if err := decodeProviderObject(data, &decoded); err != nil {
		return err
	}
	*value = PageList(decoded)
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

func decodeProviderObject(data []byte, target any) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > maxProviderObjectBytes || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("blogger: invalid provider object")
	}
	return json.Unmarshal(trimmed, target)
}

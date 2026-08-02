package lemmy

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type wirePerson struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Avatar       string `json:"avatar"`
	Banned       bool   `json:"banned"`
	Published    string `json:"published"`
	Updated      string `json:"updated"`
	ActorID      string `json:"actor_id"`
	Bio          string `json:"bio"`
	Local        bool   `json:"local"`
	Banner       string `json:"banner"`
	Deleted      bool   `json:"deleted"`
	MatrixUserID string `json:"matrix_user_id"`
	BotAccount   bool   `json:"bot_account"`
	BanExpires   string `json:"ban_expires"`
	InstanceID   int64  `json:"instance_id"`
}

type wirePersonCounts struct {
	PersonID     int64 `json:"person_id"`
	PostCount    int   `json:"post_count"`
	CommentCount int   `json:"comment_count"`
}

type wirePersonView struct {
	Person  wirePerson       `json:"person"`
	Counts  wirePersonCounts `json:"counts"`
	IsAdmin bool             `json:"is_admin"`
	Raw     json.RawMessage  `json:"-"`
}

func (view *wirePersonView) UnmarshalJSON(data []byte) error {
	type alias wirePersonView
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*view = wirePersonView(decoded)
	view.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wirePost struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	URL               string `json:"url"`
	Body              string `json:"body"`
	CreatorID         int64  `json:"creator_id"`
	CommunityID       int64  `json:"community_id"`
	Removed           bool   `json:"removed"`
	Locked            bool   `json:"locked"`
	Published         string `json:"published"`
	Updated           string `json:"updated"`
	Deleted           bool   `json:"deleted"`
	NSFW              bool   `json:"nsfw"`
	EmbedTitle        string `json:"embed_title"`
	EmbedDescription  string `json:"embed_description"`
	ThumbnailURL      string `json:"thumbnail_url"`
	APID              string `json:"ap_id"`
	Local             bool   `json:"local"`
	EmbedVideoURL     string `json:"embed_video_url"`
	LanguageID        int64  `json:"language_id"`
	FeaturedCommunity bool   `json:"featured_community"`
	FeaturedLocal     bool   `json:"featured_local"`
	URLContentType    string `json:"url_content_type"`
	AltText           string `json:"alt_text"`
}

type wireCommunity struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	Title                   string `json:"title"`
	ActorID                 string `json:"actor_id"`
	Local                   bool   `json:"local"`
	NSFW                    bool   `json:"nsfw"`
	Visibility              string `json:"visibility"`
	PostingRestrictedToMods bool   `json:"posting_restricted_to_mods"`
}

type wireImageDetails struct {
	Link        string `json:"link"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ContentType string `json:"content_type"`
}

type wirePostCounts struct {
	PostID    int64  `json:"post_id"`
	Comments  int    `json:"comments"`
	Score     int    `json:"score"`
	Upvotes   int    `json:"upvotes"`
	Downvotes int    `json:"downvotes"`
	Published string `json:"published"`
}

type wirePostView struct {
	Post         wirePost          `json:"post"`
	Creator      wirePerson        `json:"creator"`
	Community    wireCommunity     `json:"community"`
	ImageDetails *wireImageDetails `json:"image_details"`
	Counts       wirePostCounts    `json:"counts"`
	MyVote       *int              `json:"my_vote"`
	Raw          json.RawMessage   `json:"-"`
}

func (view *wirePostView) UnmarshalJSON(data []byte) error {
	type alias wirePostView
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*view = wirePostView(decoded)
	view.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wireComment struct {
	ID            int64  `json:"id"`
	CreatorID     int64  `json:"creator_id"`
	PostID        int64  `json:"post_id"`
	Content       string `json:"content"`
	Removed       bool   `json:"removed"`
	Published     string `json:"published"`
	Updated       string `json:"updated"`
	Deleted       bool   `json:"deleted"`
	APID          string `json:"ap_id"`
	Local         bool   `json:"local"`
	Path          string `json:"path"`
	Distinguished bool   `json:"distinguished"`
	LanguageID    int64  `json:"language_id"`
}

type wireCommentCounts struct {
	CommentID  int64 `json:"comment_id"`
	Score      int   `json:"score"`
	Upvotes    int   `json:"upvotes"`
	Downvotes  int   `json:"downvotes"`
	ChildCount int   `json:"child_count"`
}

type wireCommentView struct {
	Comment   wireComment       `json:"comment"`
	Creator   wirePerson        `json:"creator"`
	Post      wirePost          `json:"post"`
	Community wireCommunity     `json:"community"`
	Counts    wireCommentCounts `json:"counts"`
	MyVote    *int              `json:"my_vote"`
	Raw       json.RawMessage   `json:"-"`
}

func (view *wireCommentView) UnmarshalJSON(data []byte) error {
	type alias wireCommentView
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*view = wireCommentView(decoded)
	view.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type wirePrivateMessage struct {
	ID          int64  `json:"id"`
	CreatorID   int64  `json:"creator_id"`
	RecipientID int64  `json:"recipient_id"`
	Content     string `json:"content"`
	Deleted     bool   `json:"deleted"`
	Read        bool   `json:"read"`
	Published   string `json:"published"`
	Updated     string `json:"updated"`
	APID        string `json:"ap_id"`
	Local       bool   `json:"local"`
}

type wirePrivateMessageView struct {
	PrivateMessage wirePrivateMessage `json:"private_message"`
	Creator        wirePerson         `json:"creator"`
	Recipient      wirePerson         `json:"recipient"`
	Raw            json.RawMessage    `json:"-"`
}

func (view *wirePrivateMessageView) UnmarshalJSON(data []byte) error {
	type alias wirePrivateMessageView
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*view = wirePrivateMessageView(decoded)
	view.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type personDetailsResponse struct {
	PersonView wirePersonView `json:"person_view"`
	Posts      []wirePostView `json:"posts"`
}

type postResponse struct {
	PostView wirePostView `json:"post_view"`
}

type getPostResponse struct {
	PostView   wirePostView   `json:"post_view"`
	CrossPosts []wirePostView `json:"cross_posts"`
}

type getPostsResponse struct {
	Posts    []wirePostView `json:"posts"`
	NextPage string         `json:"next_page"`
}

type commentResponse struct {
	CommentView wireCommentView `json:"comment_view"`
}

type getCommentsResponse struct {
	Comments []wireCommentView `json:"comments"`
}

type privateMessageResponse struct {
	PrivateMessageView wirePrivateMessageView `json:"private_message_view"`
}

type privateMessagesResponse struct {
	PrivateMessages []wirePrivateMessageView `json:"private_messages"`
}

// Post preserves Lemmy community and vote semantics beside the common Post.
type Post struct {
	Common            socialhub.Post
	Title             string
	Body              string
	ExternalURL       string
	AltText           string
	CommunityID       string
	CommunityName     string
	CommunityTitle    string
	CommunityActorID  string
	ActivityPubID     string
	LanguageID        string
	Local             bool
	NSFW              bool
	Locked            bool
	Removed           bool
	Deleted           bool
	FeaturedCommunity bool
	FeaturedLocal     bool
	Score             int
	Upvotes           int
	Downvotes         int
	Comments          int
	Raw               json.RawMessage
}

// CreatePostRequest creates a link, image, or text post in one community.
type CreatePostRequest struct {
	Title              string
	CommunityID        string
	URL                string
	Body               string
	AltText            string
	MediaID            string
	NSFW               bool
	LanguageID         string
	CustomThumbnailURL string
}

// UpdatePostRequest patches only non-nil fields.
type UpdatePostRequest struct {
	Title              *string
	URL                *string
	Body               *string
	AltText            *string
	NSFW               *bool
	LanguageID         *string
	CustomThumbnailURL *string
}

// ListingType selects a federated, local, subscribed, or moderator feed.
type ListingType string

const (
	ListingAll           ListingType = "All"
	ListingLocal         ListingType = "Local"
	ListingSubscribed    ListingType = "Subscribed"
	ListingModeratorView ListingType = "ModeratorView"
)

// SortType is one API v3 post ordering.
type SortType string

const (
	SortActive         SortType = "Active"
	SortHot            SortType = "Hot"
	SortNew            SortType = "New"
	SortOld            SortType = "Old"
	SortTopDay         SortType = "TopDay"
	SortTopWeek        SortType = "TopWeek"
	SortTopMonth       SortType = "TopMonth"
	SortTopYear        SortType = "TopYear"
	SortTopAll         SortType = "TopAll"
	SortMostComments   SortType = "MostComments"
	SortNewComments    SortType = "NewComments"
	SortTopHour        SortType = "TopHour"
	SortTopSixHour     SortType = "TopSixHour"
	SortTopTwelveHour  SortType = "TopTwelveHour"
	SortTopThreeMonths SortType = "TopThreeMonths"
	SortTopSixMonths   SortType = "TopSixMonths"
	SortTopNineMonths  SortType = "TopNineMonths"
	SortControversial  SortType = "Controversial"
	SortScaled         SortType = "Scaled"
)

// FeedRequest selects an API v3 cursor-paginated post feed.
type FeedRequest struct {
	Listing       ListingType
	Sort          SortType
	Cursor        string
	MaxResults    int
	CommunityID   string
	CommunityName string
	SavedOnly     bool
	LikedOnly     bool
	DislikedOnly  bool
	ShowHidden    bool
	ShowRead      bool
	ShowNSFW      bool
}

// PostWorkflow exposes fields that the common publish request cannot express.
type PostWorkflow interface {
	CreatePost(context.Context, CreatePostRequest, ...socialhub.CallOption) (*Post, error)
	GetLemmyPost(context.Context, string, ...socialhub.CallOption) (*Post, error)
	UpdatePost(context.Context, string, UpdatePostRequest, ...socialhub.CallOption) (*Post, error)
	DeletePost(context.Context, string, ...socialhub.CallOption) error
	ListFeed(context.Context, FeedRequest, ...socialhub.CallOption) (socialhub.Page[Post], error)
}

// VoteWorkflow exposes Lemmy's downvote, neutral, and upvote scores.
type VoteWorkflow interface {
	VotePost(context.Context, string, int, ...socialhub.CallOption) error
	VoteComment(context.Context, string, int, ...socialhub.CallOption) error
}

// PrivateMessage preserves Lemmy private-message state beside the common model.
type PrivateMessage struct {
	Common        socialhub.Message
	ActivityPubID string
	Deleted       bool
	Read          bool
	Local         bool
	Raw           json.RawMessage
}

// PrivateMessageWorkflow exposes API v3 message operations as a coherent set.
type PrivateMessageWorkflow interface {
	SendPrivateMessage(context.Context, string, string, ...socialhub.CallOption) (*PrivateMessage, error)
	ListPrivateMessages(context.Context, string, string, int, ...socialhub.CallOption) (socialhub.Page[PrivateMessage], error)
	EditPrivateMessage(context.Context, string, string, ...socialhub.CallOption) (*PrivateMessage, error)
	DeletePrivateMessage(context.Context, string, ...socialhub.CallOption) error
	MarkPrivateMessageRead(context.Context, string, bool, ...socialhub.CallOption) (*PrivateMessage, error)
}

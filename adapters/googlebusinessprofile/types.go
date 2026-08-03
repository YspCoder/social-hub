package googlebusinessprofile

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// LocalPostWorkflow exposes fields that do not fit the common post contract.
type LocalPostWorkflow interface {
	CreateLocalPost(context.Context, LocalPostCreateRequest, ...socialhub.CallOption) (*LocalPost, error)
	UpdateLocalPost(context.Context, string, LocalPostPatchRequest, ...socialhub.CallOption) (*LocalPost, error)
}

// ReviewWorkflow exposes verified-location review reads and owner replies.
type ReviewWorkflow interface {
	GetReview(context.Context, string, ...socialhub.CallOption) (*Review, error)
	ListReviews(context.Context, ReviewListRequest, ...socialhub.CallOption) (ReviewPage, error)
	UpdateReviewReply(context.Context, string, string, ...socialhub.CallOption) (*ReviewReply, error)
	DeleteReviewReply(context.Context, string, ...socialhub.CallOption) error
}

// Location preserves the stable business-location fields used by common
// mapping. Raw contains the complete API response.
type Location struct {
	Name          string          `json:"name"`
	LanguageCode  string          `json:"languageCode,omitempty"`
	StoreCode     string          `json:"storeCode,omitempty"`
	LocationName  string          `json:"locationName,omitempty"`
	PrimaryPhone  string          `json:"primaryPhone,omitempty"`
	WebsiteURL    string          `json:"websiteUrl,omitempty"`
	Profile       Profile         `json:"profile,omitempty"`
	Metadata      Metadata        `json:"metadata,omitempty"`
	LocationState LocationState   `json:"locationState,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type Profile struct {
	Description string `json:"description,omitempty"`
}

type Metadata struct {
	MapsURL      string `json:"mapsUrl,omitempty"`
	NewReviewURL string `json:"newReviewUrl,omitempty"`
}

type LocationState struct {
	IsVerified     bool `json:"isVerified,omitempty"`
	IsPublished    bool `json:"isPublished,omitempty"`
	IsSuspended    bool `json:"isSuspended,omitempty"`
	IsDisconnected bool `json:"isDisconnected,omitempty"`
}

func (location *Location) UnmarshalJSON(data []byte) error {
	type alias Location
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*location = Location(decoded)
	location.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type LocalPostTopicType string

const (
	LocalPostStandard LocalPostTopicType = "STANDARD"
	LocalPostEvent    LocalPostTopicType = "EVENT"
	LocalPostOffer    LocalPostTopicType = "OFFER"
	LocalPostAlert    LocalPostTopicType = "ALERT"
)

type LocalPostState string

const (
	LocalPostRejected   LocalPostState = "REJECTED"
	LocalPostLive       LocalPostState = "LIVE"
	LocalPostProcessing LocalPostState = "PROCESSING"
	LocalPostScheduled  LocalPostState = "SCHEDULED"
	LocalPostRecurring  LocalPostState = "RECURRING"
)

type ActionType string

const (
	ActionBook      ActionType = "BOOK"
	ActionOrder     ActionType = "ORDER"
	ActionShop      ActionType = "SHOP"
	ActionLearnMore ActionType = "LEARN_MORE"
	ActionSignUp    ActionType = "SIGN_UP"
	ActionCall      ActionType = "CALL"
)

type MediaFormat string

const (
	MediaFormatPhoto MediaFormat = "PHOTO"
	MediaFormatVideo MediaFormat = "VIDEO"
)

type CallToAction struct {
	ActionType ActionType `json:"actionType"`
	URL        string     `json:"url,omitempty"`
}

type LocalPostMedia struct {
	Name         string      `json:"name,omitempty"`
	MediaFormat  MediaFormat `json:"mediaFormat,omitempty"`
	GoogleURL    string      `json:"googleUrl,omitempty"`
	ThumbnailURL string      `json:"thumbnailUrl,omitempty"`
	SourceURL    string      `json:"sourceUrl,omitempty"`
	Description  string      `json:"description,omitempty"`
}

type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type TimeOfDay struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
	Seconds int `json:"seconds,omitempty"`
	Nanos   int `json:"nanos,omitempty"`
}

type TimeInterval struct {
	StartDate Date      `json:"startDate"`
	StartTime TimeOfDay `json:"startTime"`
	EndDate   Date      `json:"endDate"`
	EndTime   TimeOfDay `json:"endTime"`
}

type LocalPostEventDetails struct {
	Title    string       `json:"title"`
	Schedule TimeInterval `json:"schedule"`
}

type LocalPostOfferDetails struct {
	CouponCode      string `json:"couponCode,omitempty"`
	RedeemOnlineURL string `json:"redeemOnlineUrl,omitempty"`
	TermsConditions string `json:"termsConditions,omitempty"`
}

// LocalPost is the typed Google resource. Raw contains the complete response.
type LocalPost struct {
	Name          string                 `json:"name"`
	ID            string                 `json:"-"`
	LanguageCode  string                 `json:"languageCode,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	CallToAction  *CallToAction          `json:"callToAction,omitempty"`
	CreateTime    *time.Time             `json:"createTime,omitempty"`
	UpdateTime    *time.Time             `json:"updateTime,omitempty"`
	ScheduledTime *time.Time             `json:"scheduledTime,omitempty"`
	Event         *LocalPostEventDetails `json:"event,omitempty"`
	State         LocalPostState         `json:"state,omitempty"`
	Media         []LocalPostMedia       `json:"media,omitempty"`
	SearchURL     string                 `json:"searchUrl,omitempty"`
	TopicType     LocalPostTopicType     `json:"topicType"`
	AlertType     string                 `json:"alertType,omitempty"`
	Offer         *LocalPostOfferDetails `json:"offer,omitempty"`
	Raw           json.RawMessage        `json:"-"`
}

func (post *LocalPost) UnmarshalJSON(data []byte) error {
	type alias LocalPost
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*post = LocalPost(decoded)
	post.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type LocalPostCreateRequest struct {
	LanguageCode  string                 `json:"languageCode,omitempty"`
	Summary       string                 `json:"summary,omitempty"`
	CallToAction  *CallToAction          `json:"callToAction,omitempty"`
	ScheduledTime *time.Time             `json:"scheduledTime,omitempty"`
	Event         *LocalPostEventDetails `json:"event,omitempty"`
	Media         []LocalPostMedia       `json:"media,omitempty"`
	TopicType     LocalPostTopicType     `json:"topicType"`
	Offer         *LocalPostOfferDetails `json:"offer,omitempty"`
}

// LocalPostPatchRequest infers updateMask from non-nil fields. A pointer to an
// empty value can be used when the API permits clearing that field.
type LocalPostPatchRequest struct {
	LanguageCode  *string                `json:"languageCode,omitempty"`
	Summary       *string                `json:"summary,omitempty"`
	CallToAction  *CallToAction          `json:"callToAction,omitempty"`
	ScheduledTime *time.Time             `json:"scheduledTime,omitempty"`
	Event         *LocalPostEventDetails `json:"event,omitempty"`
	Media         *[]LocalPostMedia      `json:"media,omitempty"`
	TopicType     *LocalPostTopicType    `json:"topicType,omitempty"`
	Offer         *LocalPostOfferDetails `json:"offer,omitempty"`
}

type StarRating string

const (
	StarOne   StarRating = "ONE"
	StarTwo   StarRating = "TWO"
	StarThree StarRating = "THREE"
	StarFour  StarRating = "FOUR"
	StarFive  StarRating = "FIVE"
)

type Reviewer struct {
	ProfilePhotoURL string `json:"profilePhotoUrl,omitempty"`
	DisplayName     string `json:"displayName,omitempty"`
	IsAnonymous     bool   `json:"isAnonymous,omitempty"`
}

type ReviewReply struct {
	Comment          string     `json:"comment"`
	UpdateTime       *time.Time `json:"updateTime,omitempty"`
	ReviewReplyState string     `json:"reviewReplyState,omitempty"`
	PolicyViolation  string     `json:"policyViolation,omitempty"`
}

type ReviewMediaItem struct {
	ThumbnailURL   string `json:"thumbnailUrl,omitempty"`
	ThumbnailLabel string `json:"thumbnailLabel,omitempty"`
	VideoURL       string `json:"videoUrl,omitempty"`
}

// Review is output-only customer content for the configured location.
type Review struct {
	Name             string            `json:"name"`
	ID               string            `json:"reviewId"`
	Reviewer         Reviewer          `json:"reviewer"`
	StarRating       StarRating        `json:"starRating"`
	Comment          string            `json:"comment,omitempty"`
	CreateTime       *time.Time        `json:"createTime,omitempty"`
	UpdateTime       *time.Time        `json:"updateTime,omitempty"`
	ReviewReply      *ReviewReply      `json:"reviewReply,omitempty"`
	ReviewMediaItems []ReviewMediaItem `json:"reviewMediaItems,omitempty"`
	Raw              json.RawMessage   `json:"-"`
}

func (review *Review) UnmarshalJSON(data []byte) error {
	type alias Review
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*review = Review(decoded)
	review.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type ReviewListRequest struct {
	Cursor     string
	MaxResults int
}

type ReviewPage struct {
	Items            []Review `json:"items"`
	NextCursor       *string  `json:"next_cursor,omitempty"`
	HasMore          bool     `json:"has_more"`
	AverageRating    float64  `json:"average_rating,omitempty"`
	TotalReviewCount int      `json:"total_review_count,omitempty"`
}

type localPostListResponse struct {
	LocalPosts    []LocalPost `json:"localPosts"`
	NextPageToken string      `json:"nextPageToken"`
}

type reviewListResponse struct {
	Reviews          []Review `json:"reviews"`
	AverageRating    float64  `json:"averageRating"`
	TotalReviewCount int      `json:"totalReviewCount"`
	NextPageToken    string   `json:"nextPageToken"`
}

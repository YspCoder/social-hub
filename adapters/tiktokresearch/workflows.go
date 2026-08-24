package tiktokresearch

import (
	"context"

	"social-hub/pkg/socialhub"
)

type queryVideosPayload struct {
	Query     Query  `json:"query"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	MaxCount  int    `json:"max_count,omitempty"`
	Cursor    uint64 `json:"cursor,omitempty"`
	SearchID  string `json:"search_id,omitempty"`
	IsRandom  *bool  `json:"is_random,omitempty"`
}

type videoPageData struct {
	Videos   []Video `json:"videos"`
	Cursor   uint64  `json:"cursor"`
	HasMore  bool    `json:"has_more"`
	SearchID string  `json:"search_id"`
}

type getUserInfoPayload struct {
	Username string `json:"username"`
}

type listCommentsPayload struct {
	VideoID   ID     `json:"video_id,omitempty"`
	CommentID ID     `json:"comment_id,omitempty"`
	MaxCount  int    `json:"max_count,omitempty"`
	Cursor    uint64 `json:"cursor,omitempty"`
}

type commentPageData struct {
	Comments []Comment `json:"comments"`
	Cursor   uint64    `json:"cursor"`
	HasMore  bool      `json:"has_more"`
}

// QueryVideos queries one page of archived public videos. A continuation
// request must pass both the previous page's Cursor and opaque SearchID.
func (client *Client) QueryVideos(ctx context.Context, input QueryVideosRequest, options ...socialhub.CallOption) (*VideoPage, error) {
	const operation = "query_videos"
	fields, err := validateQueryVideosRequest(input)
	if err != nil {
		return nil, err
	}
	payload := queryVideosPayload{
		Query: input.Query, StartDate: input.StartDate, EndDate: input.EndDate,
		MaxCount: input.MaxCount, Cursor: input.Cursor, SearchID: input.SearchID, IsRandom: input.IsRandom,
	}
	var envelope responseEnvelope[videoPageData]
	meta, raw, err := client.postJSON(ctx, operation, "/v2/research/video/query/", fields, payload, &envelope, options...)
	if err != nil {
		return nil, err
	}
	data, meta, err := requireEnvelope(operation, envelope, raw, meta, querySensitiveValues(input)...)
	if err != nil {
		return nil, err
	}
	pageSize := input.MaxCount
	if pageSize == 0 {
		pageSize = DefaultVideoPageSize
	}
	if len(data.Videos) > pageSize || data.Cursor > maximumInt64Value ||
		data.HasMore && !validOpaque(data.SearchID, 4096) ||
		data.SearchID != "" && !validOpaque(data.SearchID, 4096) {
		return nil, platformContractError(operation, "TikTok returned invalid video pagination metadata")
	}
	for _, video := range data.Videos {
		if !validVideo(video, input.Fields) {
			return nil, platformContractError(operation, "TikTok returned an invalid video record")
		}
	}
	return &VideoPage{
		Videos: data.Videos, Cursor: data.Cursor, HasMore: data.HasMore, SearchID: data.SearchID,
		Meta: meta, Raw: raw,
	}, nil
}

// GetUserInfo reads selected public fields for one username.
func (client *Client) GetUserInfo(ctx context.Context, input GetUserInfoRequest, options ...socialhub.CallOption) (*UserResponse, error) {
	const operation = "get_user_info"
	fields, err := validateGetUserInfoRequest(input)
	if err != nil {
		return nil, err
	}
	var envelope responseEnvelope[User]
	meta, raw, err := client.postJSON(
		ctx, operation, "/v2/research/user/info/", fields,
		getUserInfoPayload{Username: input.Username}, &envelope, options...,
	)
	if err != nil {
		return nil, err
	}
	user, meta, err := requireEnvelope(operation, envelope, raw, meta, input.Username)
	if err != nil {
		return nil, err
	}
	if !validUser(*user, input.Username) {
		return nil, platformContractError(operation, "TikTok returned invalid or mismatched user data")
	}
	user.Username = input.Username
	return &UserResponse{User: *user, Meta: meta, Raw: raw}, nil
}

// ListComments lists top-level comments by VideoID or replies by CommentID.
// TikTok anonymizes personal information in Research API comment records.
func (client *Client) ListComments(ctx context.Context, input ListCommentsRequest, options ...socialhub.CallOption) (*CommentPage, error) {
	const operation = "list_comments"
	fields, err := validateListCommentsRequest(input)
	if err != nil {
		return nil, err
	}
	payload := listCommentsPayload{
		VideoID: ID(input.VideoID), CommentID: ID(input.CommentID),
		MaxCount: input.MaxCount, Cursor: input.Cursor,
	}
	var envelope responseEnvelope[commentPageData]
	meta, raw, err := client.postJSON(ctx, operation, "/v2/research/video/comment/list/", fields, payload, &envelope, options...)
	if err != nil {
		return nil, err
	}
	data, meta, err := requireEnvelope(operation, envelope, raw, meta, input.VideoID, input.CommentID)
	if err != nil {
		return nil, err
	}
	pageSize := input.MaxCount
	if pageSize == 0 {
		pageSize = DefaultCommentPageSize
	}
	if len(data.Comments) > pageSize || data.Cursor > maximumInt64Value {
		return nil, platformContractError(operation, "TikTok returned too many comment records")
	}
	for _, comment := range data.Comments {
		if !validComment(comment, input.Fields) {
			return nil, platformContractError(operation, "TikTok returned an invalid comment record")
		}
	}
	return &CommentPage{Comments: data.Comments, Cursor: data.Cursor, HasMore: data.HasMore, Meta: meta, Raw: raw}, nil
}

func querySensitiveValues(input QueryVideosRequest) []string {
	values := []string{input.SearchID}
	for _, group := range [][]Condition{input.Query.And, input.Query.Or, input.Query.Not} {
		for _, condition := range group {
			values = append(values, condition.FieldValues...)
		}
	}
	return values
}

package tiktok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"unicode/utf16"

	"social-hub/pkg/socialhub"
)

const (
	minimumChunkSize        int64 = 5 << 20
	maximumChunkSize        int64 = 64 << 20
	maximumFinalChunk       int64 = 128 << 20
	maximumVideoSize        int64 = 4 << 30
	maximumUploadErrorBytes int64 = 1 << 20
)

// VideoSource identifies how TikTok receives a video.
type VideoSource string

const (
	SourceFileUpload  VideoSource = "FILE_UPLOAD"
	SourcePullFromURL VideoSource = "PULL_FROM_URL"
)

// PrivacyLevel is one choice returned by CreatorInfo.
type PrivacyLevel string

const (
	PrivacyPublic    PrivacyLevel = "PUBLIC_TO_EVERYONE"
	PrivacyMutual    PrivacyLevel = "MUTUAL_FOLLOW_FRIENDS"
	PrivacyFollowers PrivacyLevel = "FOLLOWER_OF_CREATOR"
	PrivacySelfOnly  PrivacyLevel = "SELF_ONLY"
)

// CreatorInfo contains the live choices that an export UI must show.
type CreatorInfo struct {
	AvatarURL               string         `json:"creator_avatar_url"`
	Username                string         `json:"creator_username"`
	Nickname                string         `json:"creator_nickname"`
	PrivacyLevelOptions     []PrivacyLevel `json:"privacy_level_options"`
	CommentDisabled         bool           `json:"comment_disabled"`
	DuetDisabled            bool           `json:"duet_disabled"`
	StitchDisabled          bool           `json:"stitch_disabled"`
	MaxVideoPostDurationSec int            `json:"max_video_post_duration_sec"`
}

// VideoPostRequest initializes one Direct Post video task.
type VideoPostRequest struct {
	Title            string
	PrivacyLevel     PrivacyLevel
	DisableDuet      bool
	DisableComment   bool
	DisableStitch    bool
	CoverTimestampMS int
	BrandContent     bool
	BrandOrganic     bool
	IsAIGC           bool
	Source           VideoSource
	VideoURL         string
	VideoSize        int64
	ChunkSize        int64
	TotalChunks      int
	MIME             string
}

// PublishTask reports TikTok's asynchronous publication state.
type PublishTask struct {
	ID            string
	UploadURL     string
	Status        string
	State         socialhub.PublishState
	FailReason    string
	PublicPostIDs []string
	UploadedBytes int64
}

// ContentWorkflow exposes TikTok's creator-aware Direct Post lifecycle.
type ContentWorkflow interface {
	CreatorInfo(context.Context, ...socialhub.CallOption) (*CreatorInfo, error)
	InitVideo(context.Context, VideoPostRequest, ...socialhub.CallOption) (*PublishTask, error)
	UploadChunk(context.Context, string, int, io.Reader, ...socialhub.CallOption) (*socialhub.UploadedPart, error)
	Status(context.Context, string, ...socialhub.CallOption) (*PublishTask, error)
}

// ContentService implements ContentWorkflow.
type ContentService struct{ client *Client }

type videoUpload struct {
	URL         string
	MIME        string
	VideoSize   int64
	ChunkSize   int64
	TotalChunks int
	NextPart    int
	Uploading   bool
}

func (s *ContentService) CreatorInfo(ctx context.Context, options ...socialhub.CallOption) (*CreatorInfo, error) {
	if err := s.client.requireScope("creator_info", "video.publish"); err != nil {
		return nil, err
	}
	var response creatorEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/v2/post/publish/creator_info/query/", nil, struct{}{}, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIError("creator_info", response.Error); err != nil {
		return nil, err
	}
	if len(response.Data.PrivacyLevelOptions) == 0 {
		return nil, platformError("creator_info", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data, nil
}

func (s *ContentService) InitVideo(ctx context.Context, input VideoPostRequest, options ...socialhub.CallOption) (*PublishTask, error) {
	if err := s.client.requireScope("video_init", "video.publish"); err != nil {
		return nil, err
	}
	if err := validateVideoRequest(input); err != nil {
		return nil, err
	}
	body := struct {
		PostInfo struct {
			Title            string       `json:"title,omitempty"`
			PrivacyLevel     PrivacyLevel `json:"privacy_level"`
			DisableDuet      bool         `json:"disable_duet"`
			DisableComment   bool         `json:"disable_comment"`
			DisableStitch    bool         `json:"disable_stitch"`
			CoverTimestampMS int          `json:"video_cover_timestamp_ms,omitempty"`
			BrandContent     bool         `json:"brand_content_toggle"`
			BrandOrganic     bool         `json:"brand_organic_toggle"`
			IsAIGC           bool         `json:"is_aigc,omitempty"`
		} `json:"post_info"`
		SourceInfo struct {
			Source      VideoSource `json:"source"`
			VideoURL    string      `json:"video_url,omitempty"`
			VideoSize   int64       `json:"video_size,omitempty"`
			ChunkSize   int64       `json:"chunk_size,omitempty"`
			TotalChunks int         `json:"total_chunk_count,omitempty"`
		} `json:"source_info"`
	}{}
	body.PostInfo.Title = input.Title
	body.PostInfo.PrivacyLevel = input.PrivacyLevel
	body.PostInfo.DisableDuet = input.DisableDuet
	body.PostInfo.DisableComment = input.DisableComment
	body.PostInfo.DisableStitch = input.DisableStitch
	body.PostInfo.CoverTimestampMS = input.CoverTimestampMS
	body.PostInfo.BrandContent = input.BrandContent
	body.PostInfo.BrandOrganic = input.BrandOrganic
	body.PostInfo.IsAIGC = input.IsAIGC
	body.SourceInfo.Source = input.Source
	body.SourceInfo.VideoURL = input.VideoURL
	body.SourceInfo.VideoSize = input.VideoSize
	body.SourceInfo.ChunkSize = input.ChunkSize
	body.SourceInfo.TotalChunks = input.TotalChunks
	var response publishEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/v2/post/publish/video/init/", nil, body, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIError("video_init", response.Error); err != nil {
		return nil, err
	}
	if response.Data.PublishID == "" || (input.Source == SourceFileUpload && !validEndpoint(response.Data.UploadURL)) {
		return nil, platformError("video_init", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if input.Source == SourceFileUpload {
		s.client.uploadMu.Lock()
		s.client.uploads[response.Data.PublishID] = &videoUpload{
			URL: response.Data.UploadURL, MIME: input.MIME, VideoSize: input.VideoSize,
			ChunkSize: input.ChunkSize, TotalChunks: input.TotalChunks,
		}
		s.client.uploadMu.Unlock()
	}
	return &PublishTask{ID: response.Data.PublishID, UploadURL: response.Data.UploadURL, Status: "INITIALIZED", State: socialhub.PublishStatePending}, nil
}

func validateVideoRequest(input VideoPostRequest) error {
	if !validPrivacy(input.PrivacyLevel) {
		return invalidArgument("video_init", "privacy level must be one of TikTok's creator options")
	}
	if len(utf16.Encode([]rune(input.Title))) > 2200 || input.CoverTimestampMS < 0 {
		return invalidArgument("video_init", "title exceeds 2200 UTF-16 code units or cover timestamp is negative")
	}
	switch input.Source {
	case SourcePullFromURL:
		parsed, err := url.Parse(input.VideoURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || input.VideoSize != 0 || input.ChunkSize != 0 || input.TotalChunks != 0 {
			return invalidArgument("video_init", "PULL_FROM_URL requires one absolute HTTPS URL and no chunk fields")
		}
	case SourceFileUpload:
		if input.VideoURL != "" || input.VideoSize <= 0 || input.VideoSize > maximumVideoSize || input.TotalChunks < 1 || input.TotalChunks > 1000 {
			return invalidArgument("video_init", "FILE_UPLOAD requires a video up to 4 GB and 1-1000 chunks")
		}
		if input.MIME != "video/mp4" && input.MIME != "video/quicktime" && input.MIME != "video/webm" {
			return invalidArgument("video_init", "MIME must be video/mp4, video/quicktime, or video/webm")
		}
		if input.VideoSize < minimumChunkSize {
			if input.ChunkSize != input.VideoSize || input.TotalChunks != 1 {
				return invalidArgument("video_init", "videos smaller than 5 MB must be uploaded as one whole chunk")
			}
			return nil
		}
		if input.ChunkSize < minimumChunkSize || input.ChunkSize > maximumChunkSize || int(input.VideoSize/input.ChunkSize) != input.TotalChunks {
			return invalidArgument("video_init", "chunk size must be 5-64 MB and total chunks must equal floor(video_size/chunk_size)")
		}
		if input.VideoSize > maximumChunkSize && input.TotalChunks < 2 {
			return invalidArgument("video_init", "videos larger than 64 MB require multiple chunks")
		}
		finalSize := input.VideoSize - int64(input.TotalChunks-1)*input.ChunkSize
		if finalSize <= 0 || finalSize > maximumFinalChunk {
			return invalidArgument("video_init", "final merged chunk must be no larger than 128 MB")
		}
	default:
		return invalidArgument("video_init", "source must be FILE_UPLOAD or PULL_FROM_URL")
	}
	return nil
}

func validPrivacy(value PrivacyLevel) bool {
	switch value {
	case PrivacyPublic, PrivacyMutual, PrivacyFollowers, PrivacySelfOnly:
		return true
	default:
		return false
	}
}

func (s *ContentService) UploadChunk(ctx context.Context, publishID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if publishID == "" || partNumber < 0 || reader == nil {
		return nil, invalidArgument("upload_chunk", "publish ID, non-negative part number, and reader are required")
	}
	callOptions, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	s.client.uploadMu.Lock()
	state := s.client.uploads[publishID]
	if state == nil {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_chunk", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.Uploading || partNumber != state.NextPart || partNumber >= state.TotalChunks {
		s.client.uploadMu.Unlock()
		return nil, platformError("upload_chunk", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.Uploading = true
	uploadURL, mimeType := state.URL, state.MIME
	start := int64(partNumber) * state.ChunkSize
	expectedSize := state.ChunkSize
	if partNumber == state.TotalChunks-1 {
		expectedSize = state.VideoSize - start
	}
	totalSize := state.VideoSize
	s.client.uploadMu.Unlock()

	counting := &countingReader{reader: reader}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, counting)
	if err != nil {
		s.setChunkIdle(publishID)
		return nil, platformError("upload_chunk", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", mimeType)
	request.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+expectedSize-1, totalSize))
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	request.ContentLength = expectedSize
	response, err := s.client.httpClient.Do(request)
	if err != nil {
		s.setChunkIdle(publishID)
		return nil, platformError("upload_chunk", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, sanitizeUploadError(err))
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maximumUploadErrorBytes+1))
	if readErr != nil {
		s.setChunkIdle(publishID)
		return nil, platformError("upload_chunk", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
	}
	if int64(len(body)) > maximumUploadErrorBytes {
		s.setChunkIdle(publishID)
		return nil, platformError("upload_chunk", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.setChunkIdle(publishID)
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	if counting.count != expectedSize {
		s.setChunkIdle(publishID)
		return nil, invalidArgument("upload_chunk", "uploaded byte count does not match the expected chunk size")
	}
	s.client.uploadMu.Lock()
	if current := s.client.uploads[publishID]; current != nil && current.NextPart == partNumber {
		current.NextPart++
		current.Uploading = false
	}
	s.client.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: partNumber, ETag: response.Header.Get("Content-Range"), Size: counting.count}, nil
}

func (s *ContentService) setChunkIdle(publishID string) {
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	if state := s.client.uploads[publishID]; state != nil {
		state.Uploading = false
	}
}

func sanitizeUploadError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func (s *ContentService) Status(ctx context.Context, publishID string, options ...socialhub.CallOption) (*PublishTask, error) {
	if publishID == "" {
		return nil, invalidArgument("publish_status", "publish ID is required")
	}
	if err := s.client.requireScope("publish_status", "video.publish"); err != nil {
		return nil, err
	}
	var response statusEnvelope
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/v2/post/publish/status/fetch/", nil, map[string]string{"publish_id": publishID}, &response, options...); err != nil {
		return nil, err
	}
	if err := checkAPIError("publish_status", response.Error); err != nil {
		return nil, err
	}
	state := socialhub.PublishStatePending
	switch response.Data.Status {
	case "PUBLISH_COMPLETE":
		state = socialhub.PublishStatePublished
	case "FAILED":
		state = socialhub.PublishStateFailed
	}
	return &PublishTask{
		ID: publishID, Status: response.Data.Status, State: state, FailReason: response.Data.FailReason,
		PublicPostIDs: append([]string(nil), response.Data.PubliclyAvailablePostIDs...), UploadedBytes: response.Data.UploadedBytes,
	}, nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	r.count += int64(n)
	return n, err
}

var _ ContentWorkflow = (*ContentService)(nil)

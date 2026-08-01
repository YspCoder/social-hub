package qq

import (
	"context"
	"math"
	"net/http"
	"time"

	"social-hub/pkg/socialhub"
)

type uploadURLPayload struct {
	FileType MediaFileType `json:"file_type"`
	URL      string        `json:"url"`
	Send     bool          `json:"srv_send_msg"`
	Filename string        `json:"file_name,omitempty"`
}

type uploadURLResponse struct {
	APIError
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int64  `json:"ttl"`
	RawURL   string `json:"raw_url"`
}

func (c *Client) UploadURL(ctx context.Context, input UploadURLRequest, options ...socialhub.CallOption) (*MediaAsset, error) {
	if err := validateTarget(input.Target); err != nil {
		return nil, err
	}
	if input.Target.Scene == SceneChannel {
		return nil, unsupported("upload_media_url", "QQ file_info uploads support C2C and group targets only")
	}
	if input.Type < MediaFileImage || input.Type > MediaFileFile {
		return nil, invalidArgument("upload_media_url", "media type must be image, video, voice, or file")
	}
	if !validMediaURL(input.URL) {
		return nil, invalidArgument("upload_media_url", "media URL must be absolute HTTP(S) without credentials")
	}
	if input.Filename != "" && !validBoundedString(input.Filename, 255) {
		return nil, invalidArgument("upload_media_url", "filename is invalid")
	}
	payload := uploadURLPayload{FileType: input.Type, URL: input.URL, Filename: input.Filename}
	var response uploadURLResponse
	if err := c.api.JSON(ctx, http.MethodPost, input.Target.filesPath(), nil, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := c.responseError(ctx, "upload_media_url", response.APIError); err != nil {
		return nil, err
	}
	if !validOpaque(response.FileUUID, 512) || !validBoundedString(response.FileInfo, 65536) || response.TTL < 0 || response.TTL > math.MaxInt64/int64(time.Second) {
		return nil, platformError("upload_media_url", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	asset := &MediaAsset{
		Target: input.Target, Type: input.Type, FileUUID: response.FileUUID,
		FileInfo: response.FileInfo, RawURL: response.RawURL,
	}
	if response.TTL > 0 {
		asset.TTL = time.Duration(response.TTL) * time.Second
		expiresAt := c.clock.Now().Add(asset.TTL)
		asset.ExpiresAt = &expiresAt
	}
	return asset, nil
}

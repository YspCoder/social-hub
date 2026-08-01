package officialaccount

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/extensions/material"
	"social-hub/pkg/socialhub"
)

type uploadState struct {
	request   socialhub.BeginUploadRequest
	media     *socialhub.Media
	uploading bool
}

type materialAsset struct{ asset material.Asset }

// MaterialService implements WeChat temporary and permanent material APIs.
type MaterialService struct{ client *Client }

func (s *MaterialService) Upload(ctx context.Context, kind material.Kind, mediaType socialhub.MediaType, reader io.Reader, metadata material.Metadata) (*material.Asset, error) {
	if reader == nil || metadata.Filename == "" || metadata.MIME == "" {
		return nil, invalidArgument("material_upload", "reader, filename, and MIME type are required")
	}
	typeName, err := wechatMediaType(mediaType)
	if err != nil {
		return nil, err
	}
	path := "/cgi-bin/media/upload"
	if kind == material.Permanent {
		path = "/cgi-bin/material/add_material"
	} else if kind != material.Temporary {
		return nil, invalidArgument("material_upload", "material kind is invalid")
	}
	query := url.Values{"type": {typeName}}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan error, 1)
	go func() {
		var writeErr error
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="media"; filename="%s"`, safeFilename(metadata.Filename)))
		header.Set("Content-Type", metadata.MIME)
		part, err := writer.CreatePart(header)
		if err != nil {
			writeErr = err
		} else {
			_, writeErr = io.Copy(part, reader)
		}
		if writeErr == nil && kind == material.Permanent && mediaType == socialhub.MediaTypeVideo {
			description, _ := json.Marshal(map[string]string{"title": metadata.Title, "introduction": metadata.Caption})
			writeErr = writer.WriteField("description", string(description))
		}
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	request, err := s.client.transport.NewRequest(ctx, http.MethodPost, path, query, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	var response struct {
		APIResponse
		Type      string `json:"type"`
		MediaID   string `json:"media_id"`
		CreatedAt int64  `json:"created_at"`
		URL       string `json:"url"`
	}
	err = s.client.transport.Do(request, &response)
	writeErr := <-done
	if err != nil {
		return nil, err
	}
	if writeErr != nil {
		return nil, wrapError("material_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if err := response.APIResponse.Err("material_upload"); err != nil {
		return nil, err
	}
	createdAt := time.Unix(response.CreatedAt, 0)
	if response.CreatedAt == 0 {
		createdAt = s.client.clock.Now()
	}
	asset := &material.Asset{ID: response.MediaID, Kind: kind, Type: mediaType, CreatedAt: &createdAt, URL: stringPointer(response.URL)}
	if kind == material.Temporary {
		expiresAt := createdAt.Add(3 * 24 * time.Hour)
		asset.ExpiresAt = &expiresAt
	}
	s.client.uploadMu.Lock()
	s.client.assets[asset.ID] = &materialAsset{asset: *asset}
	s.client.uploadMu.Unlock()
	return asset, nil
}

func (s *MaterialService) Get(_ context.Context, mediaID string) (*material.Asset, error) {
	if mediaID == "" {
		return nil, invalidArgument("material_get", "media ID is required")
	}
	s.client.uploadMu.Lock()
	defer s.client.uploadMu.Unlock()
	stored := s.client.assets[mediaID]
	if stored == nil {
		return nil, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "wechat", Product: "official-account", Op: "material_get", PlatformMessage: "WeChat returns file content rather than portable asset metadata; only locally observed assets can be returned"}
	}
	copy := stored.asset
	return &copy, nil
}

func (s *MaterialService) List(ctx context.Context, input material.ListRequest) (socialhub.Page[material.Asset], error) {
	if input.Kind != material.Permanent {
		return socialhub.Page[material.Asset]{}, &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "wechat", Product: "official-account", Op: "material_list", PlatformMessage: "temporary material cannot be listed"}
	}
	typeName, err := wechatMediaType(input.Type)
	if err != nil {
		return socialhub.Page[material.Asset]{}, err
	}
	offset := 0
	if input.Cursor != "" {
		offset, err = strconv.Atoi(input.Cursor)
		if err != nil || offset < 0 {
			return socialhub.Page[material.Asset]{}, invalidArgument("material_list", "cursor must be a non-negative offset")
		}
	}
	count := input.Limit
	if count <= 0 || count > 20 {
		count = 20
	}
	var response struct {
		APIResponse
		TotalCount int `json:"total_count"`
		ItemCount  int `json:"item_count"`
		Items      []struct {
			MediaID    string `json:"media_id"`
			Name       string `json:"name"`
			URL        string `json:"url"`
			UpdateTime int64  `json:"update_time"`
		} `json:"item"`
	}
	body := map[string]any{"type": typeName, "offset": offset, "count": count}
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/material/batchget_material", nil, body, &response); err != nil {
		return socialhub.Page[material.Asset]{}, err
	}
	if err := response.APIResponse.Err("material_list"); err != nil {
		return socialhub.Page[material.Asset]{}, err
	}
	items := make([]material.Asset, 0, len(response.Items))
	for _, item := range response.Items {
		updatedAt := time.Unix(item.UpdateTime, 0)
		asset := material.Asset{ID: item.MediaID, Kind: material.Permanent, Type: input.Type, CreatedAt: &updatedAt, URL: stringPointer(item.URL)}
		items = append(items, asset)
		s.client.uploadMu.Lock()
		s.client.assets[asset.ID] = &materialAsset{asset: asset}
		s.client.uploadMu.Unlock()
	}
	nextOffset := offset + len(items)
	var nextCursor *string
	if nextOffset < response.TotalCount {
		value := strconv.Itoa(nextOffset)
		nextCursor = &value
	}
	return socialhub.Page[material.Asset]{Items: items, NextCursor: nextCursor, HasMore: nextCursor != nil}, nil
}

func (s *MaterialService) Delete(ctx context.Context, mediaID string) error {
	if mediaID == "" {
		return invalidArgument("material_delete", "media ID is required")
	}
	var response APIResponse
	if err := s.client.transport.JSON(ctx, http.MethodPost, "/cgi-bin/material/del_material", nil, map[string]string{"media_id": mediaID}, &response); err != nil {
		return err
	}
	if err := response.Err("material_delete"); err != nil {
		return err
	}
	s.client.uploadMu.Lock()
	delete(s.client.assets, mediaID)
	s.client.uploadMu.Unlock()
	return nil
}

func (c *Client) BeginUpload(_ context.Context, input socialhub.BeginUploadRequest, _ ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Filename == "" || input.MIME == "" || input.Size <= 0 {
		return nil, invalidArgument("begin_upload", "filename, MIME type, and positive size are required")
	}
	if _, err := wechatMediaType(input.Type); err != nil {
		return nil, err
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return nil, wrapError("begin_upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	sessionID := "direct:" + base64.RawURLEncoding.EncodeToString(random)
	c.uploadMu.Lock()
	c.uploads[sessionID] = &uploadState{request: input}
	c.uploadMu.Unlock()
	return &socialhub.UploadSession{ID: sessionID, PartSize: input.Size}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, _ ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber != 0 || reader == nil {
		return nil, invalidArgument("upload_part", "temporary media upload requires session ID, part 0, and reader")
	}
	c.uploadMu.Lock()
	state := c.uploads[sessionID]
	if state == nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.uploading || state.media != nil {
		c.uploadMu.Unlock()
		return nil, wrapError("upload_part", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	state.uploading = true
	c.uploadMu.Unlock()
	defer func() {
		c.uploadMu.Lock()
		state.uploading = false
		c.uploadMu.Unlock()
	}()
	counting := &countingReader{reader: reader}
	asset, err := c.materials.Upload(ctx, material.Temporary, state.request.Type, counting, material.Metadata{Filename: state.request.Filename, MIME: state.request.MIME})
	if err != nil {
		return nil, err
	}
	if counting.count != state.request.Size {
		return nil, invalidArgument("upload_part", "uploaded byte count does not match declared size")
	}
	media := &socialhub.Media{ID: asset.ID, MIME: state.request.MIME, Type: state.request.Type, Size: &state.request.Size, State: socialhub.MediaStateReady, ExpiresAt: asset.ExpiresAt}
	c.uploadMu.Lock()
	state.media = media
	c.uploadMu.Unlock()
	return &socialhub.UploadedPart{Number: 0, ETag: asset.ID, Size: state.request.Size}, nil
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

func (c *Client) CompleteUpload(_ context.Context, sessionID string, parts []socialhub.UploadedPart, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" || len(parts) != 1 || parts[0].Number != 0 {
		return nil, invalidArgument("complete_upload", "exactly upload part 0 is required")
	}
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[sessionID]
	if state == nil {
		return nil, wrapError("complete_upload", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	if state.media == nil {
		return nil, wrapError("complete_upload", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	media := *state.media
	delete(c.uploads, sessionID)
	c.uploads[media.ID] = &uploadState{request: state.request, media: &media}
	return &media, nil
}

func (c *Client) MediaStatus(_ context.Context, mediaID string, _ ...socialhub.CallOption) (*socialhub.Media, error) {
	c.uploadMu.Lock()
	defer c.uploadMu.Unlock()
	state := c.uploads[mediaID]
	if state == nil || state.media == nil {
		return nil, wrapError("media_status", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	copy := *state.media
	return &copy, nil
}

func wechatMediaType(mediaType socialhub.MediaType) (string, error) {
	switch mediaType {
	case socialhub.MediaTypeImage:
		return "image", nil
	case socialhub.MediaTypeAudio:
		return "voice", nil
	case socialhub.MediaTypeVideo:
		return "video", nil
	default:
		return "", &socialhub.Error{Code: socialhub.CodeUnsupported, Class: socialhub.ClassPermanent, Platform: "wechat", Product: "official-account", Op: "media_type", PlatformMessage: "unsupported WeChat material type"}
	}
}

func safeFilename(value string) string {
	value = strings.ReplaceAll(value, "\r", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

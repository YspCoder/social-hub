package x

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

const defaultUploadPartSize int64 = 4 << 20

type mediaResponse struct {
	Data xMediaUpload `json:"data"`
}

type xMediaUpload struct {
	ID               string          `json:"id"`
	MediaKey         string          `json:"media_key"`
	Size             int64           `json:"size"`
	ExpiresAfterSecs int64           `json:"expires_after_secs"`
	ProcessingInfo   *processingInfo `json:"processing_info"`
}

type processingInfo struct {
	State           string `json:"state"`
	CheckAfterSecs  int64  `json:"check_after_secs"`
	ProgressPercent int    `json:"progress_percent"`
	Error           *struct {
		Code    int    `json:"code"`
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) BeginUpload(ctx context.Context, input socialhub.BeginUploadRequest, options ...socialhub.CallOption) (*socialhub.UploadSession, error) {
	if input.Size <= 0 || input.MIME == "" {
		return nil, invalidArgument("begin_upload", "positive size and MIME type are required")
	}
	category := input.Category
	if category == "" {
		switch input.Type {
		case socialhub.MediaTypeImage:
			category = "tweet_image"
		case socialhub.MediaTypeAnimation:
			category = "tweet_gif"
		default:
			category = "tweet_video"
		}
	}
	response, err := c.multipart(ctx, http.MethodPost, "/2/media/upload", map[string]string{
		"command":        "INIT",
		"media_type":     input.MIME,
		"total_bytes":    strconv.FormatInt(input.Size, 10),
		"media_category": category,
	}, "", "", nil, options...)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(time.Duration(response.Data.ExpiresAfterSecs) * time.Second)
	return &socialhub.UploadSession{ID: response.Data.ID, MediaID: response.Data.ID, PartSize: defaultUploadPartSize, ExpiresAt: &expiresAt}, nil
}

func (c *Client) UploadPart(ctx context.Context, sessionID string, partNumber int, reader io.Reader, options ...socialhub.CallOption) (*socialhub.UploadedPart, error) {
	if sessionID == "" || partNumber < 0 || reader == nil {
		return nil, invalidArgument("upload_part", "session ID, non-negative part number, and reader are required")
	}
	counting := &countingReader{reader: reader}
	_, err := c.multipart(ctx, http.MethodPost, "/2/media/upload", map[string]string{
		"command":       "APPEND",
		"media_id":      sessionID,
		"segment_index": strconv.Itoa(partNumber),
	}, "media", "chunk", counting, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.UploadedPart{Number: partNumber, Size: counting.count}, nil
}

func (c *Client) CompleteUpload(ctx context.Context, sessionID string, _ []socialhub.UploadedPart, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if sessionID == "" {
		return nil, invalidArgument("complete_upload", "session ID is required")
	}
	response, err := c.multipart(ctx, http.MethodPost, "/2/media/upload", map[string]string{
		"command":  "FINALIZE",
		"media_id": sessionID,
	}, "", "", nil, options...)
	if err != nil {
		return nil, err
	}
	return mapUploadedMedia(response.Data), nil
}

func (c *Client) MediaStatus(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if mediaID == "" {
		return nil, invalidArgument("media_status", "media ID is required")
	}
	query := url.Values{"command": {"STATUS"}, "media_id": {mediaID}}
	var response mediaResponse
	if err := c.transport.JSON(ctx, http.MethodGet, "/2/media/upload", query, nil, &response, options...); err != nil {
		return nil, err
	}
	return mapUploadedMedia(response.Data), nil
}

func (c *Client) multipart(ctx context.Context, method, path string, fields map[string]string, fileField, filename string, file io.Reader, options ...socialhub.CallOption) (mediaResponse, error) {
	var body io.Reader
	var bodyCloser io.Closer
	var contentType string
	var wait func() error
	if file == nil {
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		for name, value := range fields {
			if err := writer.WriteField(name, value); err != nil {
				return mediaResponse{}, err
			}
		}
		if err := writer.Close(); err != nil {
			return mediaResponse{}, err
		}
		body, contentType = &buffer, writer.FormDataContentType()
		wait = func() error { return nil }
	} else {
		pipeReader, pipeWriter := io.Pipe()
		writer := multipart.NewWriter(pipeWriter)
		contentType = writer.FormDataContentType()
		done := make(chan error, 1)
		go func() {
			var writeErr error
			for name, value := range fields {
				if writeErr = writer.WriteField(name, value); writeErr != nil {
					break
				}
			}
			if writeErr == nil {
				var part io.Writer
				part, writeErr = writer.CreateFormFile(fileField, filename)
				if writeErr == nil {
					_, writeErr = io.Copy(part, file)
				}
			}
			if closeErr := writer.Close(); writeErr == nil {
				writeErr = closeErr
			}
			_ = pipeWriter.CloseWithError(writeErr)
			done <- writeErr
		}()
		body = pipeReader
		bodyCloser = pipeReader
		wait = func() error { return <-done }
	}
	request, err := c.transport.NewRequest(ctx, method, path, nil, body, options...)
	if err != nil {
		if bodyCloser != nil {
			_ = bodyCloser.Close()
			_ = wait()
		}
		return mediaResponse{}, err
	}
	request.Header.Set("Content-Type", contentType)
	var response mediaResponse
	err = c.transport.Do(request, &response)
	writeErr := wait()
	if err != nil {
		return mediaResponse{}, err
	}
	if writeErr != nil {
		return mediaResponse{}, &socialhub.Error{Code: socialhub.CodeInvalidArgument, Class: socialhub.ClassPermanent, Platform: "x", Op: "media_upload", Cause: writeErr}
	}
	return response, nil
}

func mapUploadedMedia(input xMediaUpload) *socialhub.Media {
	state := socialhub.MediaStateReady
	if input.ProcessingInfo != nil {
		switch input.ProcessingInfo.State {
		case "pending", "in_progress":
			state = socialhub.MediaStateProcessing
		case "failed":
			state = socialhub.MediaStateFailed
		case "succeeded":
			state = socialhub.MediaStateReady
		}
	}
	media := &socialhub.Media{ID: input.ID, Size: &input.Size, State: state}
	if input.ExpiresAfterSecs > 0 {
		expiresAt := time.Now().Add(time.Duration(input.ExpiresAfterSecs) * time.Second)
		media.ExpiresAt = &expiresAt
	}
	if input.ProcessingInfo != nil {
		raw, _ := json.Marshal(input.ProcessingInfo)
		media.Extensions = map[string]json.RawMessage{"x.processing_info": raw}
	}
	return media
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
